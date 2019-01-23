package slex

import (
	"github.com/hzxiao/goutil/assert"
	"strings"
	"testing"
)

func TestParseRoute(t *testing.T) {
	r1, err := parseRoute("node1->tcp://192.168.2.2:22", 0)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(r1.nodes))
	assert.Equal(t, "tcp", r1.scheme)
	assert.Equal(t, 2, len(strings.Split(r1.destination, ":")))

	//
	r2, err := parseRoute("tcp://192.168.2.2:22", 0)
	assert.NoError(t, err)

	assert.Equal(t, 0, len(r2.nodes))
	assert.Equal(t, "tcp", r2.scheme)
	assert.Equal(t, 2, len(strings.Split(r1.destination, ":")))

	//
	_, err = parseRoute("", 0)
	assert.Error(t, err)
}

func TestLocalForward(t *testing.T) {

}
