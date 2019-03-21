package slex

import (
	"fmt"
	"github.com/hzxiao/goutil"
	"io"
)

type Conn interface {
	ReadMessage() (*Message, error)
	ReadFull(buf []byte) (int, error)
	WriteMessage(msg *Message) (int, error)
	Write(data []byte) (int, error)
	Read(buf []byte) (int, error)
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

func (c *conn) Read(buf []byte) (int, error) {
	return c.Raw.Read(buf)
}

func (c *conn) Close() error {
	return c.Raw.Close()
}

func writeJson(conn Conn, cmd byte, data goutil.Map) {
	writeJsonAndBytes(conn, cmd, data, nil)
}

func writeJsonAndBytes(conn Conn, cmd byte, info goutil.Map, data []byte) (int, error) {
	infoBytes, _ := jsonEncode(info)
	var body []byte
	body = append(body, uint32ToBytes(uint32(len(infoBytes)))...)
	body = append(body, infoBytes...)
	body = append(body, data...)

	return conn.WriteMessage(&Message{
		Cmd:  cmd,
		Body: body,
	})
}

func decodeJsonAndBytes(buf []byte) (goutil.Map, []byte, error) {
	if len(buf) < 4 {
		return nil, buf, fmt.Errorf("buf len must larger then 4")
	}

	infoLen := int(bytesToUint32(buf[:4]))
	if len(buf) < infoLen+4 {
		return nil, buf, fmt.Errorf("buf len must equal with or larger then %v", infoLen+4)
	}

	info, err := jsonDecode(buf[4 : 4+infoLen])
	if err != nil {
		return nil, buf, err
	}

	var left []byte
	if len(buf) > 4+infoLen {
		left = buf[4+infoLen:]
	}

	return info, left, err
}
