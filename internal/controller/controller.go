package controller

import (
	"encoding/gob"
	"fmt"
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

func init() {
	// gob.Register()
}

// NewController creates a new instance of the controller.
func NewController() *Controller {
	return &Controller{
		shutdown: make(chan struct{}),
	}
}

func (c *Controller) Start() {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("Controller failed to start listener: %v\n", err)
		return
	}
	defer listener.Close()

	// Go routine to handle shutdowns
	go func() {
		shutdown := <-c.shutdown
		_ = shutdown
		fmt.Printf("Shutdown received. Cleaning up....")
		listener.Close()
	}()

	// Main loop to handle connections
	fmt.Printf("Controller listener started: %v\n", listener.Addr().String())
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the error is due to listener closure
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return // Listener was closed
			}
			fmt.Printf("Error accepting connection: %v\n", err)
			return
		}

		go c.HandleClient(conn)
	}
}

// Shutdown gracefully shuts down the controller and worker nodes.
func (c *Controller) Shutdown() {
	fmt.Printf("Received Shutdown call")
	close(c.shutdown) // Close the controller's shutdown channel
}

// HandleClient handles incoming client connections for the controller.
func (c *Controller) HandleClient(conn net.Conn) {
	defer conn.Close()
	var signupRequest common.SignupRequest
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&signupRequest); err != nil {
		fmt.Printf("Error decoding message: %v\n", err)
	}
	fmt.Printf("New signup request from %v\n", signupRequest.Address)
	encoder := gob.NewEncoder(conn)
	response := common.SignupResponse{Response: common.Response{Success: true}}
	if err := encoder.Encode(response); err != nil {
		fmt.Printf("Signup error to server: %v", err)
	}
}
