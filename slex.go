package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"github.com/hzxiao/slex/conf"
	"net"
	"sync"
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

	lock sync.Mutex
}

func (s *Slex) Start() (err error) {
	if s.IsServer {
		err = s.listenAndAccept()
		if err != nil {
			log.Error("[Slex] listen and accept at %v err: %v", s.Config.Listen, err)
			return fmt.Errorf("listen and accept at %v err: %v", s.Config.Listen, err)
		}
	}

	s.EstablishChannels()
	return nil
}

func (s *Slex) EstablishChannels() (err error) {
	for _, chanOpt := range s.Config.Channels {
		channel := &Channel{
			Enable:     chanOpt.Enable,
			Token:      chanOpt.Token,
			RemoteAddr: chanOpt.RemoteAddr,
			Name:       chanOpt.RemoteAddr,
			s:          s,
			Initiator:  true,
			State:      ChanStateUnconnected,
		}

		if channel.Enable && channel.RemoteAddr != "" {
			err = channel.Connect()
			if err != nil {
				log.Error("[Slex] establish channel(%v) err: %v", channel.RemoteAddr)
				continue
			}
			go channel.loopRead()
			log.Info("[Slex] try to establish channel(%v)...", channel.RemoteAddr)
		}

		err = s.AddChannel(channel)
		if err != nil {
			log.Error("[Slex] establish and add to channel(%v) slex err: %v", channel.RemoteAddr, err)
		}
	}
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
		log.Error("[Slex] read message from raw(%v) err: %v", raw.RemoteAddr())
		log.Info("[Slex] close raw(%v)", raw.RemoteAddr())
		conn.Close()
		return
	}

	if firstMsg.Cmd != CmdChannelConnect {
		log.Warn("[Slex] recv first cmd is not 'connect' from raw（%v)", raw.RemoteAddr())
		writeJson(conn, CmdChannelConnectResp, goutil.Map{
			"result":  "fail",
			"message": "Wrong first cmd",
		})
		log.Info("[Slex] close raw(%v)", raw.RemoteAddr())
		conn.Close()
		return
	}

	data, err := jsonDecode(firstMsg.Body)
	if err != nil {
		log.Error("[Slex] decode json data(%v) from raw（%v) err: %v", firstMsg.Body, raw.RemoteAddr(), err)
		writeJson(conn, CmdChannelConnectResp, goutil.Map{
			"result":  "fail",
			"message": "Decode json error",
		})
		log.Info("[Slex] close raw(%v)", raw.RemoteAddr())
		conn.Close()
		return
	}

	ok, err := s.Auth(data)
	if err != nil {
		log.Error("[Slex] auth client by data(%v) from raw（%v) err: %v", goutil.Struct2Json(data), raw.RemoteAddr(), err)
		writeJson(conn, CmdChannelConnectResp, goutil.Map{
			"result":  "fail",
			"message": err.Error(),
		})
		log.Info("[Slex] close raw(%v)", raw.RemoteAddr())
		conn.Close()
		return
	}

	if !ok {
		log.Warn("[Slex] auth client by data(%v) from raw（%v) fail", goutil.Struct2Json(data), raw.RemoteAddr())
		writeJson(conn, CmdChannelConnectResp, goutil.Map{
			"result":  "fail",
			"message": "No Permission",
		})
		log.Info("[Slex] close raw(%v)", raw.RemoteAddr())
		conn.Close()
		return
	}

	//add a new channel
	channel := &Channel{
		Conn:       conn,
		s:          s,
		Name:       data.GetString("name"),
		Token:      data.GetString("token"),
		RemoteAddr: raw.RemoteAddr().String(),
		State:      ChanStateConnected,
	}
	err = s.AddChannel(channel)
	if err != nil {
		log.Error("[Slex] add channel by name(%v) err: %v", channel.Name)
		writeJson(conn, CmdChannelConnectResp, goutil.Map{
			"result":  "fail",
			"message": err.Error(),
		})
		log.Info("[Slex] close raw(%v)", raw.RemoteAddr())
		conn.Close()
	}
	go channel.loopRead()
}

func (s *Slex) Auth(info goutil.Map) (bool, error) {
	if !s.Config.CheckAccess(info.GetString("name"), info.GetString("token")) {
		return false, nil
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	_, exists := s.Channels[info.GetString("name")]
	if exists {
		return false, fmt.Errorf("name(%v) is exists", info.GetString("name"))
	}

	return true, nil
}

func (s *Slex) AddChannel(channel *Channel) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, found := s.Channels[channel.Name]
	if found {
		return fmt.Errorf("channel is exists")
	}

	s.Channels[channel.Name] = channel
	return nil
}
