package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"net"
	"net/url"
	"strings"
)

const (
	RouterSeparator = "->"
)

type route struct {
	raw         string
	nodes       []string //中间的路由节点
	position    int
	scheme      string
	destination string
}

//parseRoute parse raw route to route struct
//raw route format:
//		node1->node2->scheme://ip:port
//		scheme://ip:port
func parseRoute(rawRoute string, position int) (*route, error) {
	if rawRoute == "" {
		return nil, fmt.Errorf("route must not be empty")
	}

	parts := strings.Split(rawRoute, RouterSeparator)

	dest := parts[len(parts)-1]
	u, err := url.Parse(dest)
	if err != nil {
		return nil, fmt.Errorf("parse destination(%v) of route(%v) err: %v", dest, rawRoute, err)
	}

	r := &route{
		raw:         rawRoute,
		nodes:       parts[0 : len(parts)-1],
		position:    position,
		scheme:      u.Scheme,
		destination: u.Host,
	}

	return r, nil
}

func (r *route) isStartNode() bool {
	return r.position == 0
}

func (r *route) isEndNode() bool {
	return r.position == len(r.nodes)
}

func (r *route) nextNode() string {
	return r.nodes[r.position+1]
}

type Forward struct {
	Conn

	ID        string
	localAddr string
	listener  net.Listener

	routeInfo *route
	DstID     string

	s *Slex
}

func NewForward(s *Slex, localAddr, rawRoute string, position int) (*Forward, error) {
	routeInfo, err := parseRoute(rawRoute, position)
	if err != nil {
		return nil, err
	}

	f := &Forward{
		ID:        RandString(8),
		localAddr: localAddr,
		routeInfo: routeInfo,
		s:         s,
	}

	if f.routeInfo.isStartNode() {
		err = f.listenAndAccept()
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (f *Forward) listenAndAccept() (err error) {
	var network string
	switch f.routeInfo.scheme {
	case SchemeRDP, SchemeTCP, SchemeVNC:
		network = SchemeTCP
	default:
		return fmt.Errorf("unknown scheme(%v) for route", f.routeInfo.scheme)
	}
	f.listener, err = net.Listen(network, f.localAddr)
	if err != nil {
		return
	}

	var handle = func(c net.Conn) {
		f.Conn = newConn(c)

		err = f.Dial()
		if err != nil {
			log.Error("[Forward] dial raw route(%v) err: %v", f.routeInfo.raw, err)
			f.Conn.Write([]byte(fmt.Sprintf("dial raw route(%v) err: %v", f.routeInfo.raw, err)))
			f.Close()
			return
		}
	}
	go func() {
		for {
			c, err := f.listener.Accept()
			if err != nil {
				if x, ok := err.(*net.OpError); ok && x.Op == "accept" {
					break
				}
				continue
			}
			go handle(c)
		}
	}()
	return nil
}

func (f *Forward) Dial() error {
	if f.routeInfo.isEndNode() {

	} else {
		channelName := f.routeInfo.nextNode()
		channel, ok := f.s.GetChannel(channelName)
		if !ok || channel == nil {
			return fmt.Errorf("chanel(%v) not found", channelName)
		}

		writeJson(channel, CmdForwardDial, goutil.Map{
			"route": f.routeInfo.raw,
			"position": f.routeInfo.position,
			"fid": f.ID,
		})
	}
	return nil
}

func (f *Forward) Close() error {
	if f.Conn != nil {
		f.Conn.Close()
	}
	return nil
}

func (f *Forward) CloseAll() error {
	if f.Conn != nil {
		f.Conn.Close()
	}
	if f.listener != nil {
		f.listener.Close()
	}
	return nil
}
