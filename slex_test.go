package slex

import (
	"github.com/hzxiao/goutil"
	"github.com/hzxiao/goutil/assert"
	"io"
	"testing"
	"time"
)

func TestSlex_Auth(t *testing.T) {
	s := &Slex{
		Config: &Config{
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
		Config: &Config{
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
		Config: &Config{
			Name: "cli",
			Channels: []struct {
				Name   string
				Enable bool
				Token  string
				Remote string
			}{
				{
					Name:   server.Config.Name,
					Enable: true,
					Token:  "token",
					Remote: listen,
				},
			},
		},
		Channels: make(map[string]*Channel),
	}

	err := client.EstablishChannels()
	assert.NoError(t, err)

	cliChannel, _ := client.GetChannel("srv")
	assert.NotNil(t, cliChannel)

	time.Sleep(1 * time.Second)
	assert.Equal(t, ChanStateConnected, cliChannel.State)

	_, ok := server.Channels["cli"]
	assert.True(t, ok)

	//dup name fail
	client2 := &Slex{
		Config: &Config{
			Name: "cli",
			Channels: []struct {
				Name   string
				Enable bool
				Token  string
				Remote string
			}{
				{
					Name:   server.Config.Name,
					Enable: true,
					Token:  "token",
					Remote: listen,
				},
			},
		},
		Channels: make(map[string]*Channel),
	}

	err = client2.EstablishChannels()
	assert.NoError(t, err)
	cliChannel2, _ := client2.GetChannel("srv")

	assert.NotEqual(t, ChanStateFoNoPerm, cliChannel2.State)

	//access fail
	client3 := &Slex{
		Config: &Config{
			Name: "cli",
			Channels: []struct {
				Name   string
				Enable bool
				Token  string
				Remote string
			}{
				{
					Name:   server.Config.Name,
					Enable: true,
					Token:  "wrong token",
					Remote: listen,
				},
			},
		},
		Channels: make(map[string]*Channel),
	}

	err = client3.EstablishChannels()
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)
	cliChannel3, _ := client3.GetChannel("srv")
	assert.True(t, checkConnClosed(cliChannel3.Conn))

	assert.NotEqual(t, ChanStateFoNoPerm, cliChannel3.State)

}

func checkConnClosed(c Conn) bool {
	one := []byte{}
	if _, err := c.Read(one); err != io.EOF {
		return true
	}
	return false
}
