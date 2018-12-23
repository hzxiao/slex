package slex

import (
	"fmt"
	"io"
)

type Conn interface {
	ReadMessage() (*Message, error)
	ReadFull(buf []byte) (int, error)
	WriteMessage(msg *Message) (int, error)
	Write(data []byte) (int, error)
	Close() error
}

type conn struct {
	Raw io.ReadWriteCloser
}

func newConn(raw io.ReadWriteCloser) Conn {
	return &conn{Raw: raw}
}

func (c *conn) ReadMessage() (*Message, error) {
	lenBuf := make([]byte, 4)
	n, err := c.ReadFull(lenBuf)
	if err != nil {
		return nil, err
	}
	if n != len(lenBuf) {
		return nil, fmt.Errorf("read len buf error")
	}

	data := make([]byte, bytesToUint32(lenBuf))
	n, err = c.ReadFull(data)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		Cmd:  data[0],
		Body: data[1:],
	}
	return msg, nil
}

func (c *conn) ReadFull(buf []byte) (int, error) {
	got, want := 0, len(buf)
	p := buf[:want]
	for got < want {
		n, err := c.Raw.Read(p)
		if err != nil {
			return got + n, err
		}

		got += n
		if got < want {
			p = buf[got:]
			continue
		}
	}
	return got, nil
}

func (c *conn) WriteMessage(msg *Message) (int, error) {
	return c.Write(msg.Marshal())
}

func (c *conn) Write(data []byte) (int, error) {
	return c.Raw.Write(data)
}

func (c *conn) Close() error {
	return c.Raw.Close()
}
