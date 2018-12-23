package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"github.com/hzxiao/slex/conf"
	"net"
)

const (
	SchemaTCP = "tcp"
	SchemaVNC = "vnc"
	SchemaRDP = "rdp"
)

type Slex struct {
	Config   *conf.Config
	IsServer bool

	Channels map[string]*Channel
}

func (s *Slex) Start() (err error) {
	if s.IsServer {
		err = s.listenAndAccept()
		if err != nil {
			log.Error("[Slex] listen and accept at %v err: %v", s.Config.Listen, err)
			return fmt.Errorf("listen and accept at %v err: %v", s.Config.Listen, err)
		}
	}
	return nil
}

func (s *Slex) EstablishChannels() error {

	return nil
}

func (s *Slex) listenAndAccept() error {
	listen, err := net.Listen(SchemaTCP, s.Config.Listen)
	if err != nil {
		return err
	}
	log.Info("[Slex] listen at %v", s.Config.Listen)
	go func() {
		for {
			conn, err := listen.Accept()
			if err != nil {
				log.Error("[Slex] accpect conn err: %v", err)
				continue
			}

			go s.handle(conn)
		}
	}()
	return nil
}

func (s *Slex) handle(raw net.Conn) {
	conn := newConn(raw)
	firstMsg, err := conn.ReadMessage()
	if err != nil {

	}

	if firstMsg.Cmd != CmdChannelConnect {

	}

	data, err := jsonDecode(firstMsg.Body)
	if err != nil {

	}
	ok, err := s.Auth(data)
	if err != nil {

	}
	if !ok {

	}

	//add a new channel
}

func (s *Slex) Auth(info goutil.Map) (bool, error) {
	if info == nil {
		return false, fmt.Errorf("null info")
	}

	if !s.Config.CheckAccess(info.GetString("name"), info.GetString("token")) {
		return false, nil
	}

	_, exists := s.Channels[info.GetString("name")]
	if exists {
		return false, fmt.Errorf("name(%v) is exists", info.GetString("name"))
	}

	return true, nil
}
