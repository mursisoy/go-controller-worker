package worker

import (
	"encoding/gob"
	"fmt"
	"log"
	"mursisoy/wordcount/internal/common"
	"net"
)

// Worker represents the central component for failure detection.
type Worker struct {
	// workers          []int
	shutdown          chan struct{}
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
		shutdown:          make(chan struct{}),
		controllerAddress: config.ControllerAddress,
		listenAddress:     config.ListenAddress,
		id:                id,
	}
}

func (w *Worker) Start() {
	listener, err := net.Listen("tcp", w.listenAddress)
	if err != nil {
		log.Printf("Worker failed to start listener: %v\n", err)
		return
	}
	defer listener.Close()

	// Go routine to handle shutdowns
	go func() {
		shutdown := <-w.shutdown
		_ = shutdown
		log.Printf("Shutdown received. Cleaning up....\n")
		listener.Close()
	}()

	// Main loop to handle connections
	log.Printf("Worker listener started: %v\n", listener.Addr().String())

	if err := w.signup(); err != nil {
		log.Printf("Signup failed: %v. Exiting.\n", err)
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the error is due to listener closure
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return // Listener was closed
			}
			log.Printf("Error accepting connection: %v\n", err)
			return
		}

		go w.HandleClient(conn)
	}
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

// Shutdown gracefully shuts down the worker and worker nodes.
func (w *Worker) Shutdown() {
	log.Printf("Received Shutdown call")
	close(w.shutdown) // Close the worker's shutdown channel
}

// HandleClient handles incoming client connections for the worker.
func (w *Worker) HandleClient(conn net.Conn) {
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
