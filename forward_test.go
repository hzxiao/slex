package slex

import (
	"github.com/hzxiao/goutil/assert"
	"net"
	"strings"
	"testing"
)

func TestParseRoute(t *testing.T) {
	r1, err := parseRoute("node1->tcp://192.168.2.2:22", 0)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(r1.nodes))
	assert.Equal(t, "tcp", r1.scheme)
	assert.Equal(t, 2, len(strings.Split(r1.destination, ":")))

	//
	r2, err := parseRoute("tcp://192.168.2.2:22", 0)
	assert.NoError(t, err)

	assert.Equal(t, 0, len(r2.nodes))
	assert.Equal(t, "tcp", r2.scheme)
	assert.Equal(t, 2, len(strings.Split(r1.destination, ":")))

	//
	_, err = parseRoute("", 0)
	assert.Error(t, err)
}

func TestLocalForward(t *testing.T) {
	l, err := net.Listen("tcp", ":19900")
	assert.NoError(t, err)

	var handle = func(conn net.Conn) {
		//dial
		ssh,err := net.Dial("tcp", "192.168.2.74:3389")
		assert.NoError(t, err)

		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := ssh.Read(buf)
				assert.NoError(t, err)

				conn.Write(buf[:n])
			}
		}()

		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := conn.Read(buf)
				assert.NoError(t, err)

				ssh.Write(buf[:n])
			}
		}()
	}
	go func() {
		for {
			conn, err := l.Accept()
			assert.NoError(t, err)

			go handle(conn)
		}
	}()

	done := make(chan bool)
	<- done
}
