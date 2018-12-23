package slex

import (
	"github.com/hzxiao/goutil/assert"
	"testing"
)

func TestUint32ToBytes(t *testing.T) {
	//0
	b1 := uint32ToBytes(0)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, b1)

	i1 := bytesToUint32(b1)
	assert.Equal(t, uint32(0), i1)

	//1
	b2 := uint32ToBytes(1)
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x01}, b2)

	i2 := bytesToUint32(b2)
	assert.Equal(t, uint32(1), i2)

	//256
	b3 := uint32ToBytes(256)
	assert.Equal(t, []byte{0x00, 0x00, 0x01, 0x00}, b3)

	i3 := bytesToUint32(b3)
	assert.Equal(t, uint32(256), i3)
}
