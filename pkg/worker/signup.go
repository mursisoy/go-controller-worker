package worker

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/mursisoy/go-controller-worker/pkg/common"
)

func init() {
	gob.Register(common.SignupRequest{})
	gob.Register(common.SignupResponse{})
}

func (w *Worker) signup() error {

	// Connect to the server
	conn, err := net.Dial("tcp", w.controllerAddress)
	// conn.SetDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		return fmt.Errorf("error connecting to server: %v", err)
	}
	defer conn.Close()

	// Signup request start
	cc := w.clog.LogInfof("Send signup request to controller")
	signupRequest := common.SignupRequest{
		Request: common.RequestWithClock(w.pid, cc),
		Id:      w.pid,
		Address: w.listenAddress,
	}
	var request interface{} = signupRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		return fmt.Errorf("signup error to server: %v", err)
	}

	// Signup response start
	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("error decoding message: %v", err)
	}
	switch mt := response.(type) {
	case common.SignupResponse:
		signupResponse := response.(common.SignupResponse)
		if signupResponse.Success {
			w.clog.LogMergeInfof(signupResponse.Clock, "signup on controller success")
			return nil
		} else {
			return fmt.Errorf("signup error: %v", signupResponse.Message)
		}
	default:
		return fmt.Errorf("signup error, received message type %v: %v", mt, response)
	}

}
