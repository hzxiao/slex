package slex

import (
	"encoding/binary"
	"encoding/json"
	"github.com/hzxiao/goutil"
	"math/rand"
	"time"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

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
