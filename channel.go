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

	Forwards []*Forward
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
		} else {
			log.Error("[Channel] channal connect to server(%v) fail: %v", c.RemoteAddr, data.GetString("message"))
			c.Close()
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
