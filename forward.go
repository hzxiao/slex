package slex

import (
	"fmt"
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

type Forward struct {
	Conn

	localAddr string
	routeInfo *route
}

func NewForward(localAddr, route string) (*Forward, error) {

	return nil, nil
}
