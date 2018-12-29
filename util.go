package slex

import (
	"encoding/binary"
	"encoding/json"
	"github.com/hzxiao/goutil"
)

func uint32ToBytes(v uint32) []byte {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, v)
	return data
}

func bytesToUint32(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}

func jsonEncode(data goutil.Map) ([]byte, error) {
	return json.Marshal(data)
}

func jsonDecode(buf []byte) (goutil.Map, error) {
	var data = goutil.Map{}
	err := json.Unmarshal(buf, &data)
	return data, err
}
