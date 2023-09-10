package worker

import (
	"encoding/gob"
	"log"
	"mursisoy/wordcount/internal/common"
	"net"
)

// Worker represents the central component for failure detection.
type Worker struct {
	// workers          []int
	shutdown chan struct{}
	// pendingJobs      []job
	// assignedJobs     map[worker]job
	// availableWorkers []worker
}

// type job struct {
// 	id      int
// 	jobType string
// }

// type worker struct {
// 	ip   string
// 	port int
// }

// NewWorker creates a new instance of the worker.
func NewWorker() *Worker {
	return &Worker{
		shutdown: make(chan struct{}),
	}
}

func (w *Worker) Start() {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("Worker failed to start listener: %v\n", err)
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

	if ok := w.signup(common.SignupRequest{Address: listener.Addr().String()}); !ok {
		log.Fatalf("Signup failed. Exiting.\n")
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the error is due to listener closure
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return // Listener was closed
			}
			log.Fatalf("Error accepting connection: %v\n", err)
			return
		}

		go w.HandleClient(conn)
	}
}

func (w *Worker) signup(message common.SignupRequest) bool {
	// Connect to the server
	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		log.Printf("Error connecting to server: %v\n", err)
		return false
	}
	defer conn.Close()

	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(message); err != nil {
		log.Printf("Signup error to server: %v\n", err)
		return false
	}

	var signupResponse common.SignupResponse
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&signupResponse); err != nil {
		log.Printf("Error decoding message: %v\n", err)
		return false
	}
	if signupResponse.Success {
		return true
	} else {
		log.Printf("Signup error: %v\n", signupResponse.Message)
		return false
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

	// for {
	// 	// Implement your worker's server logic here
	// 	// For example, handle commands received from workers
	// }
}
