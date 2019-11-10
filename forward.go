package slex

import (
	"context"
	"fmt"
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/log"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	RouterSeparator = "->"
	RouteToRight    = "-->"
	RouteToLeft     = "<--"
)

const (
	ForwardStateDialing uint32 = iota
	ForwardStateEstablished
	ForwardStateClosed
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

func parseNextNodeByDirect(r *route, direction string) (*route, error) {
	if r == nil {
		return nil, fmt.Errorf("nil route")
	}

	next := *r
	var pos int
	switch direction {
	case RouteToRight:
		if r.isEndNode() {
			pos = r.position - 1
		} else {
			pos = r.position + 1
		}
	case RouteToLeft:
		if r.isStartNode() {
			pos = r.position + 1
		} else {
			pos = r.position - 1
		}
	default:
		return nil, fmt.Errorf("unknown direction")
	}
	next.position = pos
	return &next, nil
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

func (r *route) isEdgedNode(diraction string) bool {
	switch diraction {
	case RouteToRight:
		return r.isEndNode()
	case RouteToLeft:
		return r.isStartNode()
	}
	return false
}

func (r *route) currentNode() string {
	return r.nodes[r.position]
}

type Forward struct {
	Conn

	ID string

	routeInfo *route
	DstID     string

	ready   chan bool
	errChan chan error
	hbChan  chan bool
	s       *Slex

	state uint32
}

func NewForward(s *Slex, rawRoute string, position int) (*Forward, error) {
	routeInfo, err := parseRoute(rawRoute, position)
	if err != nil {
		return nil, err
	}

	f := &Forward{
		ID:        RandString(8),
		routeInfo: routeInfo,
		s:         s,
		ready:     make(chan bool),
		errChan:   make(chan error),
		hbChan:    make(chan bool),
	}
	return f, nil
}

//Run start s forward
func (f *Forward) Run() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	go func() {
		err = f.Dial()
		if err != nil {
			cancel()
		}
	}()

	select {
	case <-ctx.Done():
		if err == nil {
			err = ctx.Err()
		}
	case e := <-f.errChan:
		err = e
	case <-f.ready:
		go f.loopRead()
	}
	return
}

func (f *Forward) loopRead() {
	atomic.StoreUint32(&f.state, ForwardStateEstablished)

	var err error
	var channelName, direction string
	var pos int
	if f.routeInfo.isStartNode() {
		channelName, direction = f.routeInfo.nextNode(), RouteToRight
		pos = f.routeInfo.position + 1
	} else if f.routeInfo.isEndNode() {
		channelName, direction = f.routeInfo.prevNode(), RouteToLeft
		pos = f.routeInfo.position - 1
	} else {
		log.Error("[Forward] forward route info is wrong")
		return
	}

	go f.StartHeartbeat()

	log.Info("[Forward] start to read from (%v)...", f.ID)
	buf := make([]byte, 4096)
	for {
		var n int
		n, err = f.Read(buf)
		if err != nil {
			break
		}

		channel, ok := f.s.GetChannel(channelName)
		if !ok {
			log.Error("[Forward] find channel(%v) to write data forward but not found: %v", channelName)
			break
		}

		writeJsonAndBytes(channel, CmdDataForward, goutil.Map{
			"result":    "success",
			"route":     f.routeInfo.raw,
			"position":  pos,
			"fid":       f.ID,
			"dstID":     f.DstID,
			"direction": direction,
		}, buf[:n])
	}
	log.Info("[Forward] stop reading from (%v)", f.ID)
	if f.state != ForwardStateClosed {
		//
		channel, ok := f.s.GetChannel(channelName)
		if ok {
			writeJson(channel, CmdErrNotify, goutil.Map{
				"route":     f.routeInfo.raw,
				"position":  pos,
				"fid":       f.DstID,
				"direction": direction,
			})
		}
	}
}

//Dial start to dial target addr via slex node
func (f *Forward) Dial() (err error) {
	info := goutil.Map{
		"result":    "success",
		"route":     f.routeInfo.raw,
		"position":  f.routeInfo.position + 1,
		"fid":       f.ID,
		"direction": RouteToRight,
	}

	channelName := f.routeInfo.nextNode()
	return f.s.WriteToChannel(channelName, NewMessage(CmdForwardDial, info, nil))
}

func (f *Forward) DialDst() (err error) {
	// check whether the addr allowed to be dialed
	if !f.s.Config.AllowDialAddr(f.routeInfo.destination) {
		return fmt.Errorf("not allowed address")
	}

	c, err := net.DialTimeout(f.routeInfo.scheme, f.routeInfo.destination, time.Second*15)
	if err != nil {
		return err
	}

	f.Conn = newConn(c)
	go f.loopRead()
	return nil
}

func (f *Forward) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for {
			<-ticker.C
			//send heartbeat
			var channelName, direction string
			var pos int
			if f.routeInfo.isEndNode() {
				channelName = f.routeInfo.prevNode()
				pos = f.routeInfo.position - 1
				direction = RouteToLeft
			} else {
				channelName = f.routeInfo.nextNode()
				pos = f.routeInfo.position + 1
				direction = RouteToRight
			}

			channel, ok := f.s.GetChannel(channelName)
			if !ok {
				break
			}
			writeJson(channel, CmdHeartbeat, goutil.Map{
				"route":     f.routeInfo.raw,
				"direction": direction,
				"position":  pos,
				"fid":       f.ID,
				"dstID":     f.DstID,
			})
		}
	}()

	for {
		select {
		case <-time.After(10 * time.Second):
			log.Error("[Forward] src(%v) -> dst(%v) heartbeat timeout", f.ID, f.DstID)
			f.Close()
		case <-f.hbChan:
		}
	}
}

func (f *Forward) Close() error {
	if f.Conn != nil {
		f.Conn.Close()
	}
	atomic.StoreUint32(&f.state, ForwardStateClosed)
	return nil
}

type ForwardCreator struct {
	Route    *route
	Local    string
	s        *Slex
	listener net.Listener
}

func NewForwardCreator(s *Slex, rawRoute, local string) (*ForwardCreator, error) {
	r, err := parseRoute(rawRoute, 0)
	if err != nil {
		return nil, err
	}
	return &ForwardCreator{
		s:     s,
		Route: r,
		Local: local,
	}, nil
}

func (creator *ForwardCreator) listenAndAccept() (err error) {
	var network string
	switch creator.Route.scheme {
	case SchemeRDP, SchemeTCP, SchemeVNC:
		network = SchemeTCP
	default:
		return fmt.Errorf("unknown scheme(%v) for route", creator.Route.scheme)
	}
	creator.listener, err = net.Listen(network, creator.Local)
	if err != nil {
		return
	}

	log.Info("[ForwardCreator] listen local port at: %v for creating forward(%v)", creator.Local, creator.Route.raw)
	var srcHandle = func(c net.Conn) {
		f, err := creator.Create(c)
		if err != nil {
			log.Info("[ForwardCreator] create forward at port(%v) err: %v", creator.Local, err)
			return
		}
		err = creator.s.AddForward(f)
		if err != nil {
			log.Error("[ForwardCreator] add forward(%v) to slex err: %v", f.ID, err)
			f.Close()
			return
		}
		log.Info("[ForwardCreator] create forword(%v) at port(%v) for route(%v)",
			f.ID, creator.Local, creator.Route.raw)
		err = f.Run()
		if err != nil {
			f.Close()
		}
	}

	go func() {
		for {
			c, err := creator.listener.Accept()
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

func (creator *ForwardCreator) Create(raw net.Conn) (*Forward, error) {
	f, err := NewForward(creator.s, creator.Route.raw, creator.Route.position)
	if err != nil {
		return nil, err
	}
	f.Conn = newConn(raw)
	return f, nil
}
