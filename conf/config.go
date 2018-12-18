package conf

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	Name   string
	Listen string
	Access []struct {
		Name  string
		Token string
	}
	Channels []struct {
		Enable     bool
		Token      string
		RemoteAddr string
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

	return ParseYamlBytes(cfgDate)
}

func ParseYamlBytes(data []byte) (*Config, error) {
	cfg := Config{}
	err := yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
