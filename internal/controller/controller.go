package controller

import (
	"encoding/gob"
	"log"
	"mursisoy/wordcount/internal/common"
	"net"
)

// WorkerConfig configures worker. See defaults in GetDefaultConfig.
type ControllerConfig struct {
	ListenAddress string
}

// Controller represents the central component for failure detection.
type Controller struct {
	id              string
	listenAddress   string
	shutdown        chan struct{}
	workerRegistry  *workerRegistry
	failureDetector *failureDetector
	failedWorker    chan string
}

func init() {
	gob.Register(common.SignupRequest{})
	gob.Register(common.SignupResponse{})
}

// NewController creates a new instance of the controller.
func NewController(id string, config ControllerConfig) *Controller {
	return &Controller{
		listenAddress:   config.ListenAddress,
		shutdown:        make(chan struct{}),
		failureDetector: newFailureDetector(),
		workerRegistry:  newWorkerRegistry(),
		failedWorker:    make(chan string, 5),
		id:              id,
	}
}

func (c *Controller) Start() {
	// Starts a listener
	listener, err := net.Listen("tcp", c.listenAddress)
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
		case workerId := <-c.failedWorker:
			log.Printf("Worker %v failed\n", workerId)
			c.workerRegistry.deregisterWorker(workerId)
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
	case common.JobSubmitRequest:
		c.handleJobSubmitRequest(data.(common.JobSubmitRequest), conn)
	default:
		log.Printf("%v message type received but not handled", mt)
	}
}

// Handles signup requests from client
func (c *Controller) handleSignupRequest(signupRequest common.SignupRequest, conn net.Conn) {

	log.Printf("New signup request from %v\n", signupRequest.Id)

	workerInfo := workerInfo{
		id:      signupRequest.Id,
		address: signupRequest.Address,
	}

	if c.workerRegistry.registerWorker(workerInfo) {
		log.Printf("Worker %v added to available workers\n", signupRequest.Id)
	} else {
		log.Printf("Worker %v already in available workers\n", signupRequest.Id)
	}

	var signupResponse = common.SignupResponse{Response: common.Response{Success: true}}
	encoder := gob.NewEncoder(conn)
	var response interface{} = signupResponse
	if err := encoder.Encode(&response); err != nil {
		log.Printf("Signup error to server: %v", err)
	}

	// monitorWorkerRequest := common.MonitorWorkerRequest{Address: signupRequest.Address}
	c.failureDetector.watchWorker <- workerInfo
}

func (c *Controller) handleJobSubmitRequest(jobSubmitRequest common.JobSubmitRequest, conn net.Conn) {

}
