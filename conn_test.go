package slex

import (
	"bytes"
	"github.com/hzxiao/goutil/assert"
	"io"
	"sync"
	"testing"
)

type TestRaw struct {
	buf  *bytes.Buffer
	cond *sync.Cond

	lock   *sync.Mutex
	closed bool
}

func (tr *TestRaw) Read(buf []byte) (int, error) {
	if tr.closed {
		return 0, io.EOF
	}
	if tr.buf.Len() == 0 {
		tr.cond.L.Lock()
		tr.cond.Wait()
		tr.cond.L.Unlock()
	}
	if tr.closed {
		return 0, io.EOF
	}
	tr.lock.Lock()
	defer tr.lock.Unlock()

	return tr.buf.Read(buf)
}

func (tr *TestRaw) Write(buf []byte) (int, error) {
	tr.lock.Lock()
	defer tr.lock.Unlock()

	n, err := tr.buf.Write(buf)
	if err != nil {
		return 0, err
	}

	tr.cond.Signal()
	return n, nil
}

func (tr *TestRaw) Close() error {
	tr.lock.Lock()
	defer tr.lock.Unlock()
	tr.closed = true
	tr.cond.Broadcast()
	return nil
}

func TestConn_ReadFull(t *testing.T) {
	//test write 1 time
	raw := &TestRaw{
		buf:  &bytes.Buffer{},
		cond: sync.NewCond(&sync.Mutex{}),
		lock: &sync.Mutex{},
	}
	conn := newConn(raw)

	_, err := conn.Write([]byte("xx"))
	assert.NoError(t, err)

	buf := make([]byte, 2)
	_, err = conn.ReadFull(buf)
	assert.NoError(t, err)

	assert.Equal(t, []byte("xx"), buf)

	// test write 2 times
	var buf2 = make([]byte, 4)
	done := make(chan bool)
	go func() {
		_, err = conn.ReadFull(buf2)
		assert.NoError(t, err)
		done <- true
	}()

	_, err = conn.Write([]byte("12"))
	assert.NoError(t, err)

	assert.Equal(t, byte(0), buf2[2])
	assert.Equal(t, byte(0), buf2[3])

	_, err = conn.Write([]byte("34"))
	assert.NoError(t, err)

	<-done
	assert.Equal(t, []byte("1234"), buf2)

	//test return error
	var buf3 = make([]byte, 4)
	go func() {
		_, rerr := conn.ReadFull(buf3)
		assert.Error(t, rerr)
		done <- true
	}()

	err = conn.Close()
	assert.NoError(t, err)

	<-done
}

func TestConn_ReadMessage(t *testing.T) {
	raw := &TestRaw{
		buf:  &bytes.Buffer{},
		cond: sync.NewCond(&sync.Mutex{}),
	}
	conn := newConn(raw)

	var err error
	var read *Message
	done := make(chan bool)
	go func() {
		read, err = conn.ReadMessage()
		assert.NoError(t, err)
		done <- true
	}()

	msg := &Message{
		Cmd:  0x01,
		Body: make([]byte, 1024),
	}
	_, err = conn.WriteMessage(msg)
	assert.NoError(t, err)

	<-done
	assert.Equal(t, msg.Cmd, read.Cmd)
	assert.Equal(t, msg.Body, read.Body)
}
