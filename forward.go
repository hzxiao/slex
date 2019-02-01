package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	RouterSeparator = "->"
	RouteToRight    = "-->"
	RouteToLeft     = "<--"
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
	return r.position+1 == len(r.nodes)
}

func (r *route) nextNode() string {
	return r.nodes[r.position+1]
}

func (r *route) prevNode() string {
	return r.nodes[r.position-1]
}

type Forward struct {
	Conn

	ID        string
	localAddr string
	listener  net.Listener

	routeInfo *route
	SrcID     string
	DstID     string

	ready chan bool
	s     *Slex
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
		ready:     make(chan bool),
	}

	if f.routeInfo.isStartNode() {
		err = f.listenAndAccept()
		if err != nil {
			return nil, err
		}
	}
	return f, nil
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

	log.Info("[Forward] listen local port at: %v", f.localAddr)
	var srcHandle = func(c net.Conn) {
		f.Conn = newConn(c)

		err = f.Dial()
		if err != nil {
			log.Error("[Forward] dial raw route(%v) err: %v", f.routeInfo.raw, err)
			f.Conn.Write([]byte(fmt.Sprintf("dial raw route(%v) err: %v", f.routeInfo.raw, err)))
			f.Close()
			return
		}

		go func() {
			<-f.ready

			log.Info("[Forward] start to read src(%v, %v)...", f.localAddr, f.ID)
			buf := make([]byte, 4096)
			for {
				var n int
				n, err = f.Read(buf)
				if err != nil {
					break
				}

				channelName := f.routeInfo.nextNode()
				channel, ok := f.s.GetChannel(channelName)
				if !ok || channel == nil {
					log.Error("[Forward] find channel(%v) to write data forward but not found: %v", channelName)
					//TODO: close and notify
					continue
				}

				writeJsonAndBytes(channel, CmdDataForward, goutil.Map{
					"result":    "success",
					"route":     f.routeInfo.raw,
					"position":  f.routeInfo.position,
					"fid":       f.DstID,
					"direction": RouteToRight,
				}, buf[:n])
			}

			log.Info("[Forward] stop reading src(%v, %v)", f.localAddr, f.ID)
		}()
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
			go srcHandle(c)
		}
	}()
	return nil
}

func (f *Forward) Dial() (err error) {
	var channelName, fid, dstID string
	var cmd byte
	if f.routeInfo.isEndNode() {
		err = f.DialDst()
		if err != nil {
			return err
		}
		channelName = f.routeInfo.prevNode()
		fid = f.ID
		dstID = f.SrcID
		cmd = CmdForwardDialResp
	} else {
		channelName = f.routeInfo.nextNode()
		fid = f.SrcID
		cmd = CmdForwardDial
	}

	channel, ok := f.s.GetChannel(channelName)
	if !ok || channel == nil {
		return fmt.Errorf("chanel(%v) not found", channelName)
	}

	writeJson(channel, cmd, goutil.Map{
		"result":   "success",
		"route":    f.routeInfo.raw,
		"position": f.routeInfo.position,
		"fid":      fid,
		"dstID":    dstID,
	})

	//add to slex
	if f.routeInfo.isEndNode() {
		f.DstID = dstID
		f.SrcID = f.ID
		err = f.s.AddForward(f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Forward) DialDst() (err error) {
	c, err := net.DialTimeout(f.routeInfo.scheme, f.routeInfo.destination, time.Second*15)
	if err != nil {
		return err
	}

	f.Conn = newConn(c)
	log.Info("[Forward] start to read from dst(%v)...", f.routeInfo.destination)
	go func() {
		buf := make([]byte, 4096)
		for {
			var n int
			n, err = f.Read(buf)
			if err != nil {
				log.Error("[Forward] read dst(%v) err: %v", f.routeInfo.destination, err)
				break
			}

			channelName := f.routeInfo.prevNode()
			channel, ok := f.s.GetChannel(channelName)
			if !ok || channel == nil {
				log.Error("[Forward] find channel(%v) to write data back but not found: %v", channelName)
				//TODO: close and notify
				continue
			}

			writeJsonAndBytes(channel, CmdDataForward, goutil.Map{
				"result":    "success",
				"route":     f.routeInfo.raw,
				"position":  f.routeInfo.position,
				"fid":       f.DstID,
				"direction": RouteToLeft,
			}, buf[:n])
		}

		log.Info("[Forward] stop reading dst(%v)", f.routeInfo.destination)
	}()

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
