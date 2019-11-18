package tcp

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTCP(t *testing.T) {
	timeToRoomDeletion = 100 * time.Millisecond
	go Run("debug", "8281", "pass123", "8282")
	time.Sleep(100 * time.Millisecond)
	c1, banner, _, err := ConnectToTCPServer("localhost:8281", "pass123", "testRoom", 1*time.Minute)
	assert.Equal(t, banner, "8282")
	assert.Nil(t, err)
	c2, _, _, err := ConnectToTCPServer("localhost:8281", "pass123", "testRoom")
	assert.Nil(t, err)
	_, _, _, err = ConnectToTCPServer("localhost:8281", "pass123", "testRoom")
	assert.NotNil(t, err)
	_, _, _, err = ConnectToTCPServer("localhost:8281", "pass123", "testRoom", 1*time.Nanosecond)
	assert.NotNil(t, err)

	// try sending data
	assert.Nil(t, c1.Send([]byte("hello, c2")))
	var data []byte
	for {
		data, err = c2.Receive()
		if bytes.Equal(data, []byte{1}) {
			continue
		}
	}
	assert.Nil(t, err)
	assert.Equal(t, []byte("hello, c2"), data)

	assert.Nil(t, c2.Send([]byte("hello, c1")))
	for {
		data, err = c1.Receive()
		if bytes.Equal(data, []byte{1}) {
			continue
		}
	}
	assert.Nil(t, err)
	assert.Equal(t, []byte("hello, c1"), data)

	c1.Close()
	time.Sleep(300 * time.Millisecond)
}
