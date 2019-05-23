package slex

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
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

	WhiteList []*whiteList `yaml:"whiteList"`
	BlackList []string     `yaml:"blackList"`
}

type whiteList struct {
	Type      string
	Host      string   `yaml:"host"`
	Ports     []string `yaml:"ports"`
	IPNet     *net.IPNet
	PortRange portRange
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

	err = cfg.processAllowHost()
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

func (c *Config) processAllowHost() error {
	for _, white := range c.WhiteList {
		_, ipNet, err := net.ParseCIDR(white.Host)
		if err == nil {
			white.Type = "IP"
			white.IPNet = ipNet
		} else if ip := net.ParseIP(white.Host); ip != nil {
			white.Type = "IP"
			white.IPNet = &net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(32, 32),
			}
		} else {
			white.Type = "DOMAIN"
		}

	}
	return nil
}

func (c *Config) CheckAccess(name, token string) bool {
	for _, access := range c.Access {
		if access.Name == name && access.Token == token {
			return true
		}
	}
	return false
}

func (c *Config) AllowDialAddr(host string, port int) bool {
	return true
}

type portRange [][2]int

func parsePort(ps []string) (portRange, error) {
	var rg [][2]int
	for _, p := range ps {
		if !strings.Contains(p, "-") {
			p = "-" + p
		}
		//TODO spilt port range
	}

	return portRange(rg), nil
}

func (r portRange) Len() int {
	return len(r)
}

func (r portRange) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r portRange) Less(i, j int) bool {
	return r[i][0] < r[j][0]
}

func (r portRange) Merge() {

}

func (r portRange) Contains(v int) bool {

}
