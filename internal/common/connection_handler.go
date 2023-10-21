package common

import (
	"encoding/gob"
	"net"
)

type ConnectionHandlerCallback func(net.Conn)

func HandleConnections(listener net.Listener, handler connectionHandlerCallback) {
	// Main loop to handle connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the error is due to listener closure
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return // Listener was closed
			}
			return
		}
		go handler(conn)
	}
}

func DecodeMessage(conn net.Conn) (interface{}, error) {
	var data interface{}
	decoder := gob.NewDecoder(conn)
	err := decoder.Decode(&data)
	return data, err
}
