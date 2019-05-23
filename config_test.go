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
			Origin:portRange{},
			Expect:[][2]int{},
		},
		{
			Origin:portRange{{1, 2}},
			Expect:[][2]int{{1, 2}},
		},
		{
			Origin: portRange{{200, 300}, {100, 100}, {400, 400}},
			Expect: [][2]int{{100, 100}, {200, 300}, {400, 400}},
		},
		{
			Origin: portRange{{100, 150}, {200, 300}, {100, 120}},
			Expect: [][2]int{ {100, 150}, {200, 300}},
		},
		{
			Origin: portRange{{100, 120}, {120, 300}},
			Expect: [][2]int{  {100, 300}},
		},
	}

	for _, item := range tables {
		sort.Sort(item.Origin)
		pr := item.Origin.Merge()
		assert.Equal(t, item.Expect, [][2]int(pr))
	}
}

func TestPortRange_Contains(t *testing.T) {

}
