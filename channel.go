package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"io"
	"net"
	"time"
)

const (
	ChanStateUnconnected uint32 = 0
	ChanStateConnected
	ChanStateClosed
	ChanStateFoNoPerm //no permission to connect server
)

type Channel struct {
	Conn

	s          *Slex
	Enable     bool
	RemoteAddr string
	Token      string

	Initiator bool //是否建立通道发起者
	State     uint32

	Forwards []*Forward
}

func (c *Channel) Dial() (err error) {
	raw, err := net.DialTimeout(SchemaTCP, c.RemoteAddr, time.Second*15)
	if err != nil {
		return
	}
	c.Conn = newConn(raw)
	return nil
}

func (c *Channel) Connect() error {
	body, _ := jsonEncode(goutil.Map{
		"name":  c.s.Config.Name,
		"token": c.Token,
	})
	_, err := c.WriteMessage(&Message{
		Cmd:  CmdChannelConnect,
		Body: body,
	})
	return err
}

func (c *Channel) loopRead() {
	log.Info("[Channel] channel(%v) start reading", c.RemoteAddr)
	defer func() {
		log.Info("[Channel] channel(%v) stop reading", c.RemoteAddr)
	}()

	for c.State == ChanStateConnected {
		msg, err := c.ReadMessage()
		if err == io.EOF {
			c.Close()
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

	default:
	}
	return nil
}

func (c *Channel) Close() error {
	return nil
}
