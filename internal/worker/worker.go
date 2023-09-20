package worker

import (
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"mursisoy/wordcount/internal/common"
	"net"
	"sync"
)

// Worker represents the central component for failure detection.
type Worker struct {
	// workers          []int
	done              chan struct{}
	wg                sync.WaitGroup
	controllerAddress string
	listenAddress     string
	id                string
	// pendingJobs      []job
	// assignedJobs     map[worker]job
	// availableWorkers []worker
}

// WorkerConfig configures worker. See defaults in GetDefaultConfig.
type WorkerConfig struct {
	ControllerAddress string
	ListenAddress     string
}

// type job struct {
// 	id      int
// 	jobType string
// }

// type worker struct {
// 	ip   string
// 	port int
// }

func init() {
	gob.Register(common.SignupRequest{})
	gob.Register(common.SignupResponse{})
	gob.Register(common.Ping{})
	gob.Register(common.Pong{})
}

// NewWorker creates a new instance of the worker.
func NewWorker(id string, config WorkerConfig) *Worker {
	return &Worker{
		done:              make(chan struct{}),
		controllerAddress: config.ControllerAddress,
		listenAddress:     config.ListenAddress,
		id:                id,
	}
}

func (w *Worker) Done() <-chan struct{} {
	return w.done
}

func (w *Worker) Start(ctx context.Context) (net.Addr, error) {
	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", w.listenAddress)
	if err != nil {
		return nil, fmt.Errorf("worker failed to start listener: %v", err)
	}

	// Go routine to handle shutdowns
	go func() {
		<-ctx.Done()
		listener.Close()
		log.Printf("Shutdown received. Cleaning up....\n")
		w.wg.Wait()
		close(w.done)
	}()

	// Main loop to handle connections
	log.Printf("Worker listener started: %v\n", listener.Addr().String())

	if err := w.signup(); err != nil {
		return nil, fmt.Errorf("signup failed: %v. Exiting", err)
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Check if the error is due to listener closure
				if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
					log.Printf("Closed network connection: %v", err)
					return
				}
				log.Printf("Error accepting connection: %v\n", err)
				return
			}
			go w.HandleClient(conn)
		}
	}()

	return listener.Addr(), nil
}

func (w *Worker) signup() error {

	// Connect to the server
	conn, err := net.Dial("tcp", w.controllerAddress)
	// conn.SetDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		return fmt.Errorf("error connecting to server: %v", err)
	}
	defer conn.Close()

	signupRequest := common.SignupRequest{
		Id:      w.id,
		Address: w.listenAddress,
	}

	var request interface{} = signupRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		return fmt.Errorf("signup error to server: %v", err)
	}

	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("error decoding message: %v", err)
	}
	switch mt := response.(type) {
	case common.SignupResponse:
		signupResponse := response.(common.SignupResponse)
		if signupResponse.Success {
			return nil
		} else {
			return fmt.Errorf("signup error: %v", signupResponse.Message)
		}
	default:
		return fmt.Errorf("signup error, received message type %v: %v", mt, response)
	}

}

// HandleClient handles incoming client connections for the worker.
func (w *Worker) HandleClient(conn net.Conn) {
	w.wg.Add(1)
	defer w.wg.Done()
	defer conn.Close()

	// Decode the message received or fail
	var data interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&data); err != nil {
		log.Printf("Error decoding message: %v\n", err)
		return
	}

	// Switch between decoded messages
	switch mt := data.(type) {
	case common.Ping:
		w.handleHeartbeat(data.(common.Ping), conn)
	default:
		log.Printf("%v message type received but not handled", mt)
	}
}

func (w *Worker) handleHeartbeat(pingRequest common.Ping, conn net.Conn) {
	log.Printf("New ping request from %v\n", conn.RemoteAddr())
	var pongResponse = common.Pong{}
	encoder := gob.NewEncoder(conn)
	var response interface{} = pongResponse
	if err := encoder.Encode(&response); err != nil {
		log.Printf("Pong error to server: %v", err)
	}
}
