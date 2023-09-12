package controller

import (
	"encoding/gob"
	"log"
	"mursisoy/wordcount/internal/common"
	"net"
)

// Controller represents the central component for failure detection.
type Controller struct {
	// workers          []int
	shutdown chan struct{}
	// listener net.Listener
	// pendingJobs      []job
	// assignedJobs     map[worker]job
	availableWorkers map[string]struct{}
	failureDetector  *FailureDetector
	failedWorker     chan string
}

func init() {
	gob.Register(common.SignupRequest{})
	gob.Register(common.SignupResponse{})
}

// NewController creates a new instance of the controller.
func NewController() *Controller {
	return &Controller{
		shutdown:         make(chan struct{}),
		failureDetector:  newFailureDetector(),
		availableWorkers: make(map[string]struct{}),
		failedWorker:     make(chan string),
	}
}

func (c *Controller) Start() {
	// Starts a listener
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Printf("Controller failed to start listener: %v\n", err)
		return
	}
	defer listener.Close()

	go c.failureDetector.Start(c.failedWorker)

	go c.startListener(listener)

	// Go routine to handle channels
	for {
		select {
		case <-c.shutdown:
			c.failureDetector.Shutdown()
			log.Printf("Shutdown received. Cleaning up....")
			return
		case workerAddress := <-c.failedWorker:
			log.Printf("Worker %v failed\n", workerAddress)
			c.failureDetector.unwatchWorker <- workerAddress
		}
	}

}

func (c *Controller) startListener(listener net.Listener) {
	// Main loop to handle connections
	log.Printf("Controller listener started: %v\n", listener.Addr().String())
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

		go c.HandleClient(conn)
	}
}

// Shutdown gracefully shuts down the controller and worker nodes.
func (c *Controller) Shutdown() {
	log.Printf("Received Shutdown call")
	close(c.shutdown) // Close the controller's shutdown channel
}

// HandleClient handles incoming client connections for the controller.
func (c *Controller) HandleClient(conn net.Conn) {
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
	case common.SignupRequest:
		c.handleSignupRequest(data.(common.SignupRequest), conn)
	default:
		log.Printf("%v message type received but not handled", mt)
	}
}

// Handles signup requests from client
func (c *Controller) handleSignupRequest(signupRequest common.SignupRequest, conn net.Conn) {
	log.Printf("New signup request from %v\n", signupRequest.Address)

	if _, ok := c.availableWorkers[signupRequest.Address]; !ok {
		c.availableWorkers[signupRequest.Address] = struct{}{}
		log.Printf("Worker %v added to available workers\n", signupRequest.Address)
	} else {
		log.Printf("Worker %v already in available workers\n", signupRequest.Address)
	}

	var signupResponse = common.SignupResponse{Response: common.Response{Success: true}}
	encoder := gob.NewEncoder(conn)
	var response interface{} = signupResponse
	if err := encoder.Encode(&response); err != nil {
		log.Printf("Signup error to server: %v", err)
	}

	// monitorWorkerRequest := common.MonitorWorkerRequest{Address: signupRequest.Address}
	c.failureDetector.watchWorker <- signupRequest.Address
}
