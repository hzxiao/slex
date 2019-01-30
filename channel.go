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
	body, _ := jsonEncode(goutil.Map{"name": c.s.Config.Name, "token": c.Token})
	_, err = c.WriteMessage(&Message{Cmd: CmdChannelConnect, Body: body})
	if err != nil {
		atomic.StoreUint32(&c.reconnect, 1)
		return err
	}

	//read response
	msg, err := c.ReadMessage()
	if err != nil {
		atomic.StoreUint32(&c.reconnect, 1)
		return err
	}
	if msg == nil || msg.Cmd != CmdChannelConnectResp {
		return fmt.Errorf("read invalid message")
	}

	data, err := jsonDecode(msg.Body)
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
	switch msg.Cmd {
	case CmdForwardDial:
		if !c.s.IsServer {
			return fmt.Errorf("not allow to dial throught slex client mode node")
		}
		data, err := jsonDecode(msg.Body)
		if err != nil {
			return err
		}

		forward, err := NewForward(c.s, "", data.GetString("route"), int(data.GetInt64("position")+1))
		if err != nil {
			return err
		}

		forward.SrcID = data.GetString("fid")
		//dial
		err = forward.Dial()
		if err != nil {
			return fmt.Errorf("forward dial route(%v), position(%v) fail: %v", forward.routeInfo.raw, forward.routeInfo.position, err)
		}

	case CmdForwardDialResp:
		data, err := jsonDecode(msg.Body)
		if err != nil {
			return err
		}

		routeInfo, err := parseRoute(data.GetString("route"), int(data.GetInt64("position")-1))
		if err != nil {
			return err
		}

		if routeInfo.isStartNode() {
			//find forward by id
			fid := data.GetString("dstID")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("write forward dial resp to forward(%v), but not found", fid)
			}

			forward.DstID = data.GetString("fid")
			forward.ready <- true
		} else {
			channelName := routeInfo.prevNode()
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("write forward dial resp to channel(%v), but not found", channelName)
			}

			data.Set("position", routeInfo.position)
			writeJson(channel, msg.Cmd, data)
		}

	case CmdDataForward:
		info, data, err := decodeJsonAndBytes(msg.Body)
		if err != nil {
			return err
		}
		var position int
		var routeInfo *route
		var edge bool
		var channelName string
		switch info.GetString("direction") {
		case RouteToRight:
			position = int(info.GetInt64("position") + 1)
			routeInfo, err = parseRoute(info.GetString("route"), position)
			if err != nil {
				return err
			}
			if routeInfo.isEndNode() {
				edge = true
			} else {
				channelName = routeInfo.nextNode()
			}
		case RouteToLeft:
			position = int(info.GetInt64("position") - 1)
			routeInfo, err = parseRoute(info.GetString("route"), position)
			if err != nil {
				return err
			}
			if routeInfo.isStartNode() {
				edge = true
			} else {
				channelName = routeInfo.prevNode()
			}
		default:
			return fmt.Errorf("unknown direction")
		}

		if edge {
			fid := info.GetString("fid")
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

			info.Set("position", routeInfo.position)
			writeJsonAndBytes(channel, msg.Cmd, info, data)
		}
	default:
		return fmt.Errorf("unknown cmd(%v)", msg.Body)
	}
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
