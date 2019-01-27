package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
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

type Channel struct {
	Conn

	s          *Slex
	Name       string
	Enable     bool
	RemoteAddr string
	Token      string

	Initiator bool //是否建立通道发起者
	State     uint32
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
		return err
	}
	body, _ := jsonEncode(goutil.Map{
		"name":  c.s.Config.Name,
		"token": c.Token,
	})
	_, err = c.WriteMessage(&Message{
		Cmd:  CmdChannelConnect,
		Body: body,
	})
	return err
}

func (c *Channel) loopRead() {
	var err error
	log.Info("[Channel] channel(%v) start reading", c.RemoteAddr)
	defer func() {
		if err != nil {
			log.Error("[Channel] channel(%v) stop reading for err: %v", c.RemoteAddr, err)
		} else {
			log.Info("[Channel] channel(%v) stop reading", c.RemoteAddr)
		}
	}()

	for {
		var msg *Message
		msg, err = c.ReadMessage()
		if err != nil {
			return
		}

		err = c.Handle(msg)
		if err != nil {
			log.Error("[Channel] handle message on chaneel(%v) err: %v", c.RemoteAddr, err)
		}
	}
}

func (c *Channel) Handle(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("invalid message: null msg")
	}
	switch msg.Cmd {
	case CmdChannelConnectResp:
		data, err := jsonDecode(msg.Body)
		if err != nil {
			return err
		}
		if data.GetString("result") == "success" {
			log.Info("[Channel] channel connect to server(%v) success", c.RemoteAddr)
			atomic.StoreUint32(&c.State, ChanStateConnected)
			c.Name = data.GetString("name")

			err = c.s.MoveConnectedChannel(c)
			if err != nil {
				log.Error("[Channel] move connected channel(%v) err: %v", c.Name, err)
				c.Close()
			}
		} else {
			log.Error("[Channel] channel connect to server(%v) fail: %v", c.RemoteAddr, data.GetString("message"))
			c.Close()
		}
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

		routeInfo, err := parseRoute(info.GetString("route"), int(info.GetInt64("position")+1))
		if err != nil {
			return err
		}

		if routeInfo.isEndNode() {
			fid := info.GetString("fid")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("write forward data to forward(%v), but not found", fid)
			}
			forward.Write(data)
		} else {
			channelName := routeInfo.nextNode()
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("write forward data to channel(%v), but not found", channelName)
			}

			info.Set("position", routeInfo.position)
			writeJsonAndBytes(channel, msg.Cmd, info, data)
		}
	case CmdDataBackwards:
		info, data, err := decodeJsonAndBytes(msg.Body)
		if err != nil {
			return err
		}

		routeInfo, err := parseRoute(info.GetString("route"), int(info.GetInt64("position")-1))
		if err != nil {
			return err
		}

		if routeInfo.isStartNode() {
			fid := info.GetString("fid")
			forward, ok := c.s.GetForward(fid)
			if !ok {
				return fmt.Errorf("write backwards data to forward(%v), but not found", fid)
			}
			forward.Write(data)
		} else {
			channelName := routeInfo.prevNode()
			channel, ok := c.s.GetChannel(channelName)
			if !ok {
				return fmt.Errorf("write backwards data to channel(%v), but not found", channelName)
			}

			info.Set("position", routeInfo.position)
			writeJsonAndBytes(channel, msg.Cmd, info, data)
		}
	default:
		c.Close()
		return fmt.Errorf("unknown cmd(%v)", msg.Body)
	}
	return nil
}

func (c *Channel) Close() error {
	return c.Conn.Close()
}
