package slex

const (
	CmdChannelConnect = 0x01
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
