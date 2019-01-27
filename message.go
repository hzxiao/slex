package slex

const (
	CmdChannelConnect     = 0x01
	CmdChannelConnectResp = 0x02
	CmdForwardDial        = 0x03
	CmdForwardDialResp    = 0x04
	CmdDataForward        = 0x05
	CmdDataBackwards      = 0x06
)

type Message struct {
	Cmd  byte
	Body []byte
}

func (msg *Message) Marshal() []byte {
	length := uint32(1 + len(msg.Body))
	data := uint32ToBytes(length)
	data = append(data, msg.Cmd)
	data = append(data, msg.Body...)
	return data
}
