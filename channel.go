package slex

import (
	"crypto/tls"
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"io"
	"sync/atomic"
	"time"
)

const (
	ChanStateUnconnected uint32 = iota
	ChanStateConnected
	ChanStateClosed
	ChanStateFoNoPerm //no permission to connect server
)

const reconnectDur = time.Second * 5

type Channel struct {
	Conn

	s          *Slex
	Name       string
	Enable     bool
	RemoteAddr string
	Token      string

	Initiator bool //是否建立通道发起者
	State     uint32
	reconnect uint32 //1: need to reconnect to the server
}

//Dial connect to server
func (c *Channel) Dial() (err error) {
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	
	raw, err := tls.Dial(SchemeTCP, c.RemoteAddr, conf)
	if err != nil {
		return
	}
	c.Conn = newConn(raw)
	return nil
}

//Connect connect to server to auth the channel
func (c *Channel) Connect() (err error) {
	err = c.Dial()
	if err != nil {
		atomic.StoreUint32(&c.reconnect, 1)
		return err
	}

	//send auth message
	writeJson(c, CmdChannelConnect, goutil.Map{"name": c.s.Config.Name, "token": c.Token})

	//read response
	msg, err := c.ReadMessage()
	if err != nil {
		atomic.StoreUint32(&c.reconnect, 1)
		return err
	}
	if msg == nil || msg.Cmd != CmdChannelConnectResp {
		return fmt.Errorf("read invalid message")
	}

	data, _, err := decodeJsonAndBytes(msg.Body)
	if err != nil {
		return err
	}
	if data.GetString("result") == "success" {
		c.Name = data.GetString("name")
		atomic.StoreUint32(&c.State, ChanStateConnected)
		log.Info("[Channel] channel connect to server(%v, %v) success", c.Name, c.RemoteAddr)
	} else {
		err = fmt.Errorf(data.GetString("message"))
		log.Error("[Channel] channel connect to server(%v) fail: %v", c.RemoteAddr, err)
		if data.GetBool("forbid") {
			atomic.StoreUint32(&c.State, ChanStateFoNoPerm)
		} else {
			atomic.StoreUint32(&c.reconnect, 1)
		}
		c.Close()
	}

	if err == nil {
		go c.loopRead()
	}
	return err
}

func (c *Channel) Reconnect() {
	ticker := time.NewTicker(reconnectDur)
	defer ticker.Stop()

	for atomic.LoadUint32(&c.reconnect) == 1 {
		log.Info("[Channel] try to reconnect server(%v)...", c.RemoteAddr)
		err := c.Connect()
		if err == nil {
			atomic.StoreUint32(&c.reconnect, 0)
			break
		} else {
			log.Error("[Channel] channel reconnect server(%v) err: %v", c.RemoteAddr, err)
		}
		<-ticker.C
	}
}

func (c *Channel) loopRead() {
	var err error
	log.Info("[Channel] channel(%v, %v) start reading", c.Name, c.RemoteAddr)

	for {
		var msg *Message
		msg, err = c.ReadMessage()
		if err != nil {
			break
		}

		err = c.Handle(msg)
		if err != nil {
			log.Error("[Channel] handle message on channel(%v) err: %v", c.RemoteAddr, err)
			if msg.Cmd != CmdErrNotify {
				err = c.NotifyError(msg, err)
				if err != nil {
					log.Error("[Forward] notify error info err: %v", err)
				}
			}
		}
	}

	if err != nil {
		log.Error("[Channel] channel(%v) stop reading for err: %v", c.RemoteAddr, err)
	} else {
		log.Info("[Channel] channel(%v) stop reading", c.RemoteAddr)
	}
	if c.Initiator && err == io.EOF {
		atomic.StoreUint32(&c.reconnect, 1)
	}

	c.Close()
}

//Handle handle channel message
func (c *Channel) Handle(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("invalid message: null msg")
	}
	//decode message body
	info, data, err := decodeJsonAndBytes(msg.Body)
	if err != nil {
		return err
	}
	//current node info
	current, err := parseRoute(info.GetString("route"), int(info.GetInt64("position")))
	if err != nil {
		return err
	}
	//next node info by direction
	next, err := parseNextNodeByDirect(current, info.GetString("direction"))
	if err != nil {
		return err
	}

	if msg.Cmd == CmdForwardDial && !c.s.Config.Relay  {
		return fmt.Errorf("not support relay")
	}

	if !current.isEdgedNode(info.GetString("direction")) {
		info.Set("position", next.position)
		err = c.s.WriteToChannel(next.currentNode(), NewMessage(msg.Cmd, info, data))
		if err != nil {
			return fmt.Errorf("relay message of cmd(%v) to channel(%v) err: %v", msg.Cmd, next.currentNode(), err)
		}
		return nil
	}

	switch msg.Cmd {
	case CmdForwardDial:
		if current.isEndNode() {
			forward, err := NewForward(c.s, info.GetString("route"), int(info.GetInt64("position")))
			if err != nil {
				return err
			}
			if err = forward.DialDst(); err != nil {
				return err
			}
			forward.DstID = info.GetString("fid")

			err = forward.s.AddForward(forward)
			if err != nil {
				return err
			}
			respInfo := goutil.Map{
				"result":    "success",
				"route":     current.raw,
				"position":  next.position,
				"fid":       forward.ID,
				"dstID":     forward.DstID,
				"direction": RouteToLeft,
			}
			err = c.s.WriteToChannel(next.currentNode(), NewMessage(CmdForwardDialResp, respInfo, nil))
			if err != nil {
				return err
			}
		}
	case CmdForwardDialResp:
		if current.isStartNode() {
			fid := info.GetString("dstID")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("write forward dial resp to forward(%v), but not found", fid)
			}

			forward.DstID = info.GetString("fid")
			forward.ready <- true
		}
	case CmdDataForward:
		fid := info.GetString("dstID")
		forward, ok := c.s.GetForward(fid)
		if !ok {
			return fmt.Errorf("write forward data to forward(%v), but not found", fid)
		}
		forward.Write(data)

	case CmdErrNotify:
		fid := info.GetString("fid")
		forward := c.s.DeleteForward(fid)
		if forward != nil {
			forward.Close()
		}

	case CmdHeartbeat:
		fid := info.GetString("dstID")
		forward, ok := c.s.GetForward(fid)
		if !ok {
			return fmt.Errorf("check heartbeat to forward(%v), but not found", fid)
		}
		//send heartbeat response
		if atomic.LoadUint32(&forward.state) == ForwardStateEstablished {
			var direction string
			if info.GetString("direction") == RouteToRight {
				direction = RouteToLeft
			} else {
				direction = RouteToRight
			}

			respInfo := goutil.Map{
				"result":    "success",
				"route":     info.GetString("route"),
				"position":  next.position,
				"fid":       fid,
				"dstID":     info.GetString("fid"),
				"direction": direction,
			}
			err = c.s.WriteToChannel(next.currentNode(), NewMessage(CmdHeartbeatResp, respInfo, nil))
			return err 
		}
	case CmdHeartbeatResp:
		fid := info.GetString("dstID")
		forward, ok := c.s.GetForward(fid)
		if !ok {
			return fmt.Errorf("check heartbeat to forward(%v), but not found", fid)
		}
		forward.hbChan <- true
	default:
		return fmt.Errorf("unknown cmd(%v)", msg.Body)
	}
	return nil
}

func (c *Channel) NotifyError(msg *Message, cause error) error {
	if msg == nil {
		return fmt.Errorf("invalid message: null msg")
	}
	info, _, err := decodeJsonAndBytes(msg.Body)
	if err != nil {
		return err
	}
	routeInfo, err := parseRoute(info.GetString("route"), int(info.GetInt64("position")))
	if err != nil {
		return err
	}

	var channelName, direction string
	var pos int
	switch {
	case msg.Cmd == CmdForwardDial ||
		(msg.Cmd == CmdDataForward && info.GetString("direction") == RouteToRight):
		channelName = routeInfo.prevNode()
		pos = routeInfo.position - 1
		direction = RouteToLeft
	case msg.Cmd == CmdForwardDialResp ||
		(msg.Cmd == CmdDataForward && info.GetString("direction") == RouteToLeft):
		channelName = routeInfo.nextNode()
		pos = routeInfo.position + 1
		direction = RouteToRight
	case msg.Cmd == CmdErrNotify:
		return nil
	}

	info.Set("massage", cause)
	info.Set("position", pos)
	info.Set("direction", direction)
	channel, ok := c.s.GetChannel(channelName)
	if !ok {
		return fmt.Errorf("find channel(%v) to notify error info, but not found", channelName)
	}
	writeJson(channel, CmdErrNotify, info)
	return nil
}

func (c *Channel) Close() error {
	c.Conn.Close()

	atomic.StoreUint32(&c.State, ChanStateClosed)
	if atomic.LoadUint32(&c.reconnect) == 1 {
		c.Reconnect()
		return nil
	}

	if !c.Initiator {
		c.s.DeleteChannel(c.Name)
	}
	return nil
}
