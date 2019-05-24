package slex

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	Name   string
	Listen string
	Relay  bool
	Access []struct {
		Name  string
		Token string
	}
	Channels []struct {
		Name   string
		Enable bool
		Token  string
		Remote string
	}
	Forwards []struct {
		Local string
		Route string
	}

	WhiteList []*Addr `yaml:"whiteList"`
	BlackList []*Addr `yaml:"blackList"`
}

type Addr struct {
	Type      string
	Host      string   `yaml:"host"`
	Ports     []string `yaml:"ports"`
	IPNet     *net.IPNet
	PortRange portRange
}

func (addr *Addr) parseHost() {
	_, ipNet, err := net.ParseCIDR(addr.Host)
	if err == nil {
		addr.Type = "IP"
		addr.IPNet = ipNet
	} else if ip := net.ParseIP(addr.Host); ip != nil {
		addr.Type = "IP"
		addr.IPNet = &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(32, 32),
		}
	} else {
		addr.Type = "DOMAIN"
	}
}

func (addr *Addr) parsePort() error {
	var err error
	addr.PortRange, err = parsePortRange(addr.Ports)
	return err
}

func (addr *Addr) Match(host string, port int) bool {
	if addr.Type == "IP" {
		ip := net.ParseIP(host)
		if ip == nil {
			return false
		}
		return addr.IPNet.Contains(ip) && addr.PortRange.Contains(port)
	}

	return addr.Host == host && addr.PortRange.Contains(port)
}

func (addr *Addr) MatchHost(host string) bool {
	if addr.Type == "IP" {
		ip := net.ParseIP(host)
		if ip == nil {
			return false
		}
		return addr.IPNet.Contains(ip)
	}

	return addr.Host == host
}

func ParseConfig(filename string) (*Config, error) {
	cfgDate, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg, err := ParseYamlBytes(cfgDate)
	if err != nil {
		return nil, err
	}

	err = cfg.check()
	if err != nil {
		return nil, err
	}

	err = cfg.processAddr()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func ParseYamlBytes(data []byte) (*Config, error) {
	cfg := Config{}
	err := yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) check() error {
	//check name
	if c.Name == "" {
		return fmt.Errorf("config: name can not be empty")
	}

	err := c.checkAndFormatForward()
	if err != nil {
		return err
	}
	return nil
}

func (c *Config) checkAndFormatForward() error {
	for i, f := range c.Forwards {
		if f.Local == "" {
			return fmt.Errorf("forward local is empty at index %v", i)
		}

		if f.Route == "" {
			return fmt.Errorf("forward route is empty at index %v", i)
		}

		parts := strings.Split(f.Route, "->")
		if len(parts) < 2 {
			return fmt.Errorf("invalid forward route at index %v", i)
		}

		c.Forwards[i].Route = fmt.Sprintf("%v->%v", c.Name, f.Route)
	}

	return nil
}

func (c *Config) processAddr() (err error) {
	for _, addr := range c.WhiteList {
		addr.parseHost()
		err = addr.parsePort()
		if err != nil {
			return
		}
	}
	for _, addr := range c.BlackList {
		addr.parseHost()
		err = addr.parsePort()
		if err != nil {
			return
		}
	}
	return
}

func (c *Config) CheckAccess(name, token string) bool {
	for _, access := range c.Access {
		if access.Name == name && access.Token == token {
			return true
		}
	}
	return false
}

//AllowDialAddr check whether addr can be dialed
// host can be ip or domain name
// port list 8080 or :8080
func (c *Config) AllowDialAddr(host string, port string) bool {
	port = strings.TrimPrefix(port, ":")
	p, err := strconv.Atoi(port)
	if err != nil {
		return false
	}
	if p < 0 || p > 65535 {
		return false
	}
	for _, addr := range c.BlackList {
		if addr.MatchHost(host) {
			return false
		}
	}
	for _, addr := range c.WhiteList {
		if addr.Match(host, p) {
			return true
		}
	}
	return false
}

type portRange [][2]int

func parsePortRange(ps []string) (portRange, error) {
	var rg = make([][2]int, 0)
	for _, p := range ps {
		if p == "" {
			return nil, fmt.Errorf("empty port")
		}
		if !strings.Contains(p, "-") {
			p = p + "-" + p
		}

		parts := strings.SplitN(p, "-", 2)
		low, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid port: %v", err)
		}
		high, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid port: %v", err)
		}
		if low > high {
			return nil, fmt.Errorf("wrong port range")
		}
		rg = append(rg, [2]int{low, high})
	}

	pr := portRange(rg)
	pr = pr.Merge()
	return pr, nil
}

func (r portRange) Len() int {
	return len(r)
}

func (r portRange) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r portRange) Less(i, j int) bool {
	if r[i][0] == r[j][0] {
		return r[i][1] < r[j][1]
	}
	return r[i][0] < r[j][0]
}

func (r portRange) Merge() portRange {
	if r == nil {
		return nil
	}

	var rg = make([][2]int, len(r))
	copy(rg, r)

	if len(r) < 2 {
		return rg
	}

	var deleteIndex = func(s [][2]int, index int) [][2]int {
		if len(s) <= index {
			return s
		}
		if index == len(s)-1 {
			return s[:index]
		}
		return append(s[:index], s[index+1:]...)
	}

	sort.Sort(portRange(rg))

	for i := 0; i < len(rg)-1; {
		j := i + 1
		// (x1, y1) (x2, y2)
		x1, y1, x2, y2 := rg[i][0], rg[i][1], rg[j][0], rg[j][1]
		if x1 == x2 {
			rg[i][1] = y2 // y1 = y2
			rg = deleteIndex(rg, j)
			continue
		}
		// x1 < x2
		switch {
		case y1 < x2:
			i++
		case y1 == x2:
			rg[i][1] = y2
			rg = deleteIndex(rg, j)
		case y1 <= y2: //y1 > x2 && y1 <= y2
			rg[i][1] = y2
			rg = deleteIndex(rg, j)
		default: //y1 > x2 && y1 > y2
			rg = deleteIndex(rg, j)
		}
	}
	return rg
}

func (r portRange) Contains(v int) bool {
	for _, rg := range r {
		if v >= rg[0] && v <= rg[1] {
			return true
		}
	}
	return false
}
