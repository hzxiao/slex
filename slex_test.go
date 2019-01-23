package slex

import (
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/assert"
	"github.com/hzxiao/slex/conf"
	"io"
	"testing"
	"time"
)

func TestSlex_Auth(t *testing.T) {
	s := &Slex{
		Config: &conf.Config{
			Access: []struct {
				Name  string
				Token string
			}{
				{
					Name:  "name1",
					Token: "token1",
				},
			},
		},
		IsServer: true,
		Channels: make(map[string]*Channel),
	}

	ok, err := s.Auth(goutil.Map{"name": "name1", "token": "token1"})
	assert.NoError(t, err)
	assert.True(t, ok)

	s.Channels["name1"] = &Channel{}

	//dup
	ok, err = s.Auth(goutil.Map{"name": "name1", "token": "token1"})
	assert.Error(t, err)
	assert.False(t, ok)

	//auth fail
	ok, err = s.Auth(goutil.Map{"name": "name2", "token": "token2"})
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestSlex_AddChannel(t *testing.T) {
	s := &Slex{
		Channels: make(map[string]*Channel),
	}

	channel := &Channel{
		Name: "name",
	}

	err := s.AddChannel(channel)
	assert.NoError(t, err)

	err = s.AddChannel(channel)
	assert.Error(t, err)
}

func TestSlex_EstablishChannels(t *testing.T) {
	listen := "localhost:2203"
	server := &Slex{
		Config: &conf.Config{
			Name:   "srv",
			Listen: listen,
			Access: []struct {
				Name  string
				Token string
			}{
				{
					Name:  "cli",
					Token: "token",
				},
			},
		},
		IsServer: true,
		Channels: make(map[string]*Channel),
	}

	start := make(chan bool)
	go func() {
		err := server.Start()
		assert.NoError(t, err)
		start <- true
	}()

	<-start
	//success
	client := &Slex{
		Config: &conf.Config{
			Name: "cli",
			Channels: []struct {
				Enable     bool
				Token      string
				RemoteAddr string
			}{
				{
					Enable:     true,
					Token:      "token",
					RemoteAddr: listen,
				},
			},
		},
		Channels: make(map[string]*Channel),
	}

	err := client.EstablishChannels()
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)

	cliChannel := client.Channels[listen]
	assert.NotNil(t, cliChannel)
	assert.Equal(t, ChanStateConnected, cliChannel.State)

	_, ok := server.Channels["cli"]
	assert.True(t, ok)

	//dup name fail
	client2 := &Slex{
		Config: &conf.Config{
			Name: "cli",
			Channels: []struct {
				Enable     bool
				Token      string
				RemoteAddr string
			}{
				{
					Enable:     true,
					Token:      "token",
					RemoteAddr: listen,
				},
			},
		},
		Channels: make(map[string]*Channel),
	}

	err = client2.EstablishChannels()
	assert.NoError(t, err)
	assert.NotEqual(t, ChanStateConnected, client2.Channels[listen].State)

	//access fail
	client3 := &Slex{
		Config: &conf.Config{
			Name: "cli",
			Channels: []struct {
				Enable     bool
				Token      string
				RemoteAddr string
			}{
				{
					Enable:     true,
					Token:      "wrong token",
					RemoteAddr: listen,
				},
			},
		},
		Channels: make(map[string]*Channel),
	}

	err = client3.EstablishChannels()
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)
	cliChannel3 := client3.Channels[listen]
	assert.True(t, checkConnClosed(cliChannel3.Conn))
	assert.NotEqual(t, ChanStateConnected, client3.Channels[listen].State)

}

func checkConnClosed(c Conn) bool {
	one := []byte{}
	if _, err := c.Read(one); err != io.EOF {
		return true
	}
	return false
}
