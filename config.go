package slex

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
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

func (c *Config) CheckAccess(name, token string) bool {
	for _, access := range c.Access {
		if access.Name == name && access.Token == token {
			return true
		}
	}
	return false
}
