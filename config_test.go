package slex

import (
	"github.com/hzxiao/goutil/assert"
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
