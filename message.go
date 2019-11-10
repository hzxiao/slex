package slex

import (
	"github.com/hzxiao/goutil"
)

const (
	CmdChannelConnect     = 0x01
	CmdChannelConnectResp = 0x02
	CmdForwardDial        = 0x03
	CmdForwardDialResp    = 0x04
	CmdDataForward        = 0x05
	CmdErrNotify          = 0x06
	CmdHeartbeat          = 0x07
	CmdHeartbeatResp      = 0x08
)

type Message struct {
	Cmd  byte
	Body []byte
}

//NewMessage new a message
func NewMessage(cmd byte, info goutil.Map, data []byte) *Message {
	infoBytes, _ := jsonEncode(info)
	var body []byte
	body = append(body, uint32ToBytes(uint32(len(infoBytes)))...)
	body = append(body, infoBytes...)
	body = append(body, data...)

	return &Message{
		Cmd:  cmd,
		Body: body,
	}
}

func (msg *Message) Marshal() []byte {
	length := uint32(1 + len(msg.Body))
	data := uint32ToBytes(length)
	data = append(data, msg.Cmd)
	data = append(data, msg.Body...)
	return data
}
