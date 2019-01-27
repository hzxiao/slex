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
	SchemeTCP = "tcp"
	SchemeVNC = "vnc"
	SchemeRDP = "rdp"
)

type Slex struct {
	Config   *conf.Config
	IsServer bool

	ConnectingChannels []*Channel
	Channels           map[string]*Channel

	Forwards map[string]*Forward

	lock sync.Mutex
}

func NewSlex(config *conf.Config, isServer bool) *Slex {
	return &Slex{
		Config:   config,
		IsServer: isServer,
		Channels: make(map[string]*Channel),
		Forwards: make(map[string]*Forward),
	}
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
	err = s.InitForwards()
	if err != nil {
		return fmt.Errorf("init forwards err: %v", err)
	}
	return nil
}

func (s *Slex) EstablishChannels() (err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, chanOpt := range s.Config.Channels {
		channel := &Channel{
			Enable:     chanOpt.Enable,
			Token:      chanOpt.Token,
			RemoteAddr: chanOpt.Remote,
			s:          s,
			Initiator:  true,
			State:      ChanStateUnconnected,
		}

		if channel.Enable && channel.RemoteAddr != "" {
			err = channel.Connect()
			if err != nil {
				log.Error("[Slex] establish channel(%v) err: %v", channel.RemoteAddr, err)
				continue
			}
			go channel.loopRead()
			log.Info("[Slex] try to establish channel(%v)...", channel.RemoteAddr)
		}

		s.ConnectingChannels = append(s.ConnectingChannels, channel)
	}
	return nil
}

func (s *Slex) InitForwards() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, forwardOpt := range s.Config.Forwards {
		forward, err := NewForward(s, forwardOpt.Local, forwardOpt.Route, 0)
		if err != nil {
			return err
		}

		forward.SrcID = forward.ID
		s.Forwards[forward.ID] = forward
	}
	return nil
}

func (s *Slex) listenAndAccept() error {
	listen, err := net.Listen(SchemeTCP, s.Config.Listen)
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
			"forbid":  true,
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
			"forbid":  true,
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
			"forbid":  true,
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
			"forbid":  true,
		})
		log.Info("[Slex] close raw(%v)", raw.RemoteAddr())
		conn.Close()
	}

	writeJson(conn, CmdChannelConnectResp, goutil.Map{
		"result": "success",
		"name":   s.Config.Name,
	})
	log.Info("[Slex] auth success and add a new channel(%v), addr(%v)", data.GetString("name"), raw.RemoteAddr())

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

func (s *Slex) MoveConnectedChannel(channel *Channel) error {
	s.lock.Lock()
	var i int
	for i = 0; i < len(s.ConnectingChannels); i++ {
		if s.ConnectingChannels[i] == channel {
			break
		}
	}
	if i >= len(s.ConnectingChannels) {
		return fmt.Errorf("channel not found")
	}
	//delete channel from connecting channels slice
	if i == len(s.ConnectingChannels)-1 {
		s.ConnectingChannels = s.ConnectingChannels[:i]
	} else {
		s.ConnectingChannels = append(s.ConnectingChannels[:i], s.ConnectingChannels[i+1:]...)
	}
	s.lock.Unlock()

	return s.AddChannel(channel)
}

func (s *Slex) GetChannel(name string) (*Channel, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	c, ok := s.Channels[name]
	return c, ok
}

func (s *Slex) AddForward(f *Forward) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	_, found := s.Forwards[f.ID]
	if found {
		return fmt.Errorf("forward is exists")
	}

	s.Forwards[f.ID] = f
	return nil
}

func (s *Slex) GetForward(fid string) (*Forward, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	f, ok := s.Forwards[fid]
	return f, ok
}
