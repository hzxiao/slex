package slex

import (
	"github.com/hzxiao/goutil/assert"
	"sort"
	"testing"
)

func TestParseConfig(t *testing.T) {
	c, err := ParseConfig("./conf/slex.yaml")
	assert.NoError(nil, err)
	assert.NotNil(t, c)
}

func TestParseYamlBytes(t *testing.T) {
	confString1 := `
name: Server
listen: :2303
access:
  -
    name: Client
    token: 123456
channels:
  -
    enable: true
    token: 123456
    remote: 192.168.2.30:2303
forwards:
  -
    local: :8900
    route: Server->tcp://127.0.0.1:22
  -
    local: :8901
    route: Server->tcp://192.168.2.66:22
`

	c1, err := ParseYamlBytes([]byte(confString1))
	assert.NoError(t, err)

	assert.Equal(t, "Server", c1.Name)
	assert.Equal(t, ":2303", c1.Listen)
	assert.Len(t, c1.Access, 1)
	assert.Len(t, c1.Channels, 1)
	assert.Len(t, c1.Forwards, 2)

	confString2 := `
name: Client
#listen: :2303
channels:
  -
    enable: true
    token: 123456
    remote: 192.168.2.30:2303
forwards:
  -
    local: :8900
    route: Server->tcp://127.0.0.1:22
  -
    local: :8901
    route: Server->tcp://192.168.2.66:22
`

	c2, err := ParseYamlBytes([]byte(confString2))
	assert.NoError(t, err)

	assert.Equal(t, "Client", c2.Name)
	assert.Equal(t, "", c2.Listen)
	assert.Len(t, c2.Access, 0)
	assert.Len(t, c2.Channels, 1)
	assert.Len(t, c2.Forwards, 2)
}

func TestParsePortRange(t *testing.T) {
	pr, err := parsePortRange([]string{"100", "200-300", "400"})
	assert.NoError(t, err)
	assert.Equal(t, [][2]int{{100, 100}, {200, 300}, {400, 400}}, [][2]int(pr))

	pr, err = parsePortRange(nil)
	assert.NoError(t, err)
	assert.NotNil(t, pr)

	pr, err = parsePortRange([]string{"1", "2-1"})
	assert.Error(t, err)

	_, err = parsePortRange([]string{""})
	assert.Error(t, err)
}

func TestPortRange_Sort(t *testing.T) {
	var tables = []struct {
		Origin portRange
		Expect [][2]int
	}{
		{
			Origin: portRange{{200, 300}, {100, 100}, {400, 400}},
			Expect: [][2]int{{100, 100}, {200, 300}, {400, 400}},
		},
		{
			Origin: portRange{{100, 150}, {200, 300}, {100, 120}},
			Expect: [][2]int{{100, 120}, {100, 150}, {200, 300}},
		},
	}

	for _, item := range tables {
		sort.Sort(item.Origin)
		assert.Equal(t, item.Expect, [][2]int(item.Origin))
	}
}

func TestPortRange_Merge(t *testing.T) {
	var tables = []struct {
		Origin portRange
		Expect [][2]int
	}{
		{
			Origin: nil,
			Expect: nil,
		},
		{
			Origin: portRange{},
			Expect: [][2]int{},
		},
		{
			Origin: portRange{{1, 2}},
			Expect: [][2]int{{1, 2}},
		},
		{
			Origin: portRange{{200, 300}, {100, 100}, {400, 400}},
			Expect: [][2]int{{100, 100}, {200, 300}, {400, 400}},
		},
		{
			Origin: portRange{{100, 150}, {200, 300}, {100, 120}},
			Expect: [][2]int{{100, 150}, {200, 300}},
		},
		{
			Origin: portRange{{100, 120}, {120, 300}},
			Expect: [][2]int{{100, 300}},
		},
		{
			Origin: portRange{{100, 300}, {120, 150}},
			Expect: [][2]int{{100, 300}},
		},
		{
			Origin: portRange{{100, 130}, {120, 150}},
			Expect: [][2]int{{100, 150}},
		},
	}

	for _, item := range tables {
		sort.Sort(item.Origin)
		pr := item.Origin.Merge()
		assert.Equal(t, item.Expect, [][2]int(pr))
	}
}

func TestPortRange_Contains(t *testing.T) {
	var tables = []struct{
		portRange portRange
		Value int
		Result bool
	} {
		{
			portRange: portRange{{200, 300}, {100, 100}, {400, 400}},
			Value: 100,
			Result: true,
		},
		{
			portRange: portRange{{200, 300}, {100, 100}, {400, 400}},
			Value: 101,
			Result: false,
		},
	}

	for _, item := range tables {
		if item.Result {
			assert.True(t, item.portRange.Contains(item.Value))
		} else {
			assert.False(t, item.portRange.Contains(item.Value))
		}
	}
}

func TestAddr_MatchHost(t *testing.T) {
	var tables = []struct{
		Host string
		Value string
		Result bool
	} {
		{"172.1.0.1", "172.1.0.1", true},
		{"172.1.0.1", "172.1.0.2", false},
		{"172.1.0.1/24", "172.1.0.2", true},
		{"localhost", "172.1.0.2", false},
		{"localhost", "localhost", true},
	}

	for _, item := range tables {
		addr := &Addr{Host: item.Host}
		addr.parseHost()
		if item.Result {
			assert.True(t, addr.MatchHost(item.Value))
		} else {
			assert.False(t, addr.MatchHost(item.Value))
		}
	}
}

func TestAddr_Match(t *testing.T) {
	var tables = []struct{
		Host string
		Ports []string
		CheckHost string
		CHeckPort int
		Result bool
	} {
		{"172.1.0.1", []string{"1000-2000"}, "172.1.0.1", 1000,true},
		{"172.1.0.1", []string{"1000-2000"}, "172.1.0.2", 1000,false},
		{"172.1.0.1", []string{"1000-2000"}, "172.1.0.1", 3000,false},
		{"172.1.0.0/24", []string{"1000-2000"}, "172.1.0.2", 1000,true},
		{"localhost", []string{"1000-2000"}, "localhost", 1000,true},
		{"localhost", []string{"1000-2000"}, "localhost", 3000,false},
		{"localhost", []string{"1000-2000"}, "localhost2", 1000,false},
	}

	for _, item := range tables {
		addr := &Addr{Host: item.Host, Ports: item.Ports}
		addr.parseHost()
		err := addr.parsePort()
		assert.NoError(t, err)
		if item.Result {
			assert.True(t, addr.Match(item.CheckHost, item.CHeckPort))
		} else {
			assert.False(t, addr.Match(item.CheckHost, item.CHeckPort))
		}
	}
}

func TestConfig_AllowDialAddr(t *testing.T) {
	c, err := ParseConfig("./testdata/addr-list.yaml")
	assert.NoError(nil, err)
	assert.NotNil(t, c)

	assert.True(t, c.AllowDialAddr("192.168.10.1", "3000"))
	assert.True(t, c.AllowDialAddr("192.168.20.2", "3000"))
	assert.False(t, c.AllowDialAddr("192.168.20.1", "3000"))
	assert.True(t, c.AllowDialAddr("localhost", "3000"))
	assert.False(t, c.AllowDialAddr("192.168.20.255", "3000"))
}