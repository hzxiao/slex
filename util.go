package slex

import (
	"encoding/binary"
	"encoding/json"
	"github.com/hzxiao/goutil"
	"math/rand"
	"time"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandString(n int) string {
	b := make([]byte, n)
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
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
