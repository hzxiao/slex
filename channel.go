package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"io"
	"net"
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

func (c *Channel) Dial() (err error) {
	raw, err := net.DialTimeout(SchemeTCP, c.RemoteAddr, time.Second*15)
	if err != nil {
		return
	}
	c.Conn = newConn(raw)
	return nil
}

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
			err = c.NotifyError(msg, err)
			if err != nil {
				log.Error("[Forward] notify error info err: %v", err)
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

func (c *Channel) Handle(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("invalid message: null msg")
	}
	info, data, err := decodeJsonAndBytes(msg.Body)
	if err != nil {
		return err
	}
	routeInfo, err := parseRoute(info.GetString("route"), int(info.GetInt64("position")))
	if err != nil {
		return err
	}
	switch msg.Cmd {
	case CmdForwardDial:
		forward, err := NewForward(c.s, info.GetString("route"), int(info.GetInt64("position")))
		if err != nil {
			return err
		}

		if !forward.routeInfo.isEndNode() && !c.s.Config.Relay {
			return fmt.Errorf("not support rely")
		}

		forward.SrcID = info.GetString("fid")
		//dial
		err = forward.Dial()
		if err != nil {
			return fmt.Errorf("forward dial route(%v), position(%v) fail: %v", forward.routeInfo.raw, forward.routeInfo.position, err)
		}

	case CmdForwardDialResp:
		if routeInfo.isStartNode() {
			fid := info.GetString("dstID")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("write forward dial resp to forward(%v), but not found", fid)
			}

			forward.DstID = info.GetString("fid")
			forward.ready <- true
		} else {
			channelName := routeInfo.prevNode()
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("write forward dial resp to channel(%v), but not found", channelName)
			}

			info.Set("position", routeInfo.position-1)
			writeJson(channel, msg.Cmd, info)
		}

	case CmdDataForward:
		var (
			edge        bool
			channelName string
			pos         int
		)
		switch info.GetString("direction") {
		case RouteToRight:
			if routeInfo.isEndNode() {
				edge = true
			} else {
				channelName = routeInfo.nextNode()
				pos = routeInfo.position + 1
			}
		case RouteToLeft:
			if routeInfo.isStartNode() {
				edge = true
			} else {
				channelName = routeInfo.prevNode()
				pos = routeInfo.position - 1
			}
		default:
			return fmt.Errorf("unknown direction")
		}

		if edge {
			fid := info.GetString("dstID")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("write forward data to forward(%v), but not found", fid)
			}
			forward.Write(data)
		} else {
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("write forward data to channel(%v), but not found", channelName)
			}

			info.Set("position", pos)
			writeJsonAndBytes(channel, msg.Cmd, info, data)
		}
	case CmdErrNotify:
		var (
			edge        bool
			channelName string
			pos         int
		)
		switch info.GetString("direction") {
		case RouteToRight:
			if routeInfo.isEndNode() {
				edge = true
			} else {
				channelName = routeInfo.nextNode()
				pos = routeInfo.position + 1
			}
		case RouteToLeft:
			if routeInfo.isStartNode() {
				edge = true
			} else {
				channelName = routeInfo.prevNode()
				pos = routeInfo.position - 1
			}
		default:
			return fmt.Errorf("unknown direction")
		}

		if edge {
			fid := info.GetString("fid")
			forward := c.s.DeleteForward(fid)
			if forward != nil {
				forward.Close()
			}
		} else {
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("notify error info to channel(%v), but not found", channelName)
			}

			info.Set("position", pos)
			writeJsonAndBytes(channel, msg.Cmd, info, data)
		}
	case CmdHeartbeat:
		var (
			edge        bool
			channelName, direction string
			pos         int
		)
		switch info.GetString("direction") {
		case RouteToRight:
			if routeInfo.isEndNode() {
				edge = true
				channelName = routeInfo.prevNode()
				pos = routeInfo.position - 1
				direction = RouteToLeft
			} else {
				channelName = routeInfo.nextNode()
				pos = routeInfo.position + 1
			}
		case RouteToLeft:
			if routeInfo.isStartNode() {
				edge = true
				channelName = routeInfo.nextNode()
				pos = routeInfo.position + 1
				direction = RouteToRight
			} else {
				channelName = routeInfo.prevNode()
				pos = routeInfo.position - 1
			}
		default:
			return fmt.Errorf("unknown direction")
		}

		if edge {
			fid := info.GetString("dstID")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("check heartbeat to forward(%v), but not found", fid)
			}
			//send heartbeat response
			if atomic.LoadUint32(&forward.state) == ForwardStateEstablished {
				channel, ok := c.s.GetChannel(channelName)
				if !ok {
					return fmt.Errorf("check heartbeat to channel(%v), but not found", channelName)
				}

				writeJson(channel, CmdHeartbeatResp, goutil.Map{
					"result": "success",
					"route":     info.GetString("route"),
					"position":  pos,
					"fid":       fid,
					"dstID":     info.GetString("fid"),
					"direction": direction,
				})
			}
		} else {
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("check heartbeat to channel(%v), but not found", channelName)
			}

			info.Set("position", pos)
			writeJson(channel, msg.Cmd, info)
		}
	case CmdHeartbeatResp:
		var (
			edge        bool
			channelName string
			pos         int
		)
		switch info.GetString("direction") {
		case RouteToRight:
			if routeInfo.isEndNode() {
				edge = true
			} else {
				channelName = routeInfo.nextNode()
				pos = routeInfo.position + 1
			}
		case RouteToLeft:
			if routeInfo.isStartNode() {
				edge = true
			} else {
				channelName = routeInfo.prevNode()
				pos = routeInfo.position - 1
			}
		default:
			return fmt.Errorf("unknown direction")
		}

		if edge {
			fid := info.GetString("dstID")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("check heartbeat to forward(%v), but not found", fid)
			}
			forward.hbChan <- true
		} else {
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("send heartheaat info to channel(%v), but not found", channelName)
			}

			info.Set("position", pos)
			writeJsonAndBytes(channel, msg.Cmd, info, data)
		}
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
