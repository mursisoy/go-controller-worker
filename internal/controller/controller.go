package controller

import (
	"encoding/gob"
	"fmt"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/common"
	"net"
)

// WorkerConfig configures worker. See defaults in GetDefaultConfig.
type ControllerConfig struct {
	ListenAddress string
	LogPriority   clock.LogPriority
}

// Controller represents the central component for failure detection.
type Controller struct {
	pid             string
	listenAddress   string
	shutdown        chan struct{}
	done            chan struct{}
	workerRegistry  *workerRegistry
	failureDetector *failureDetector
	failedWorker    chan failedWorkerRequest
	clock           *clock.Clock
	log             *clock.ClockLogger
}

func init() {
	gob.Register(common.SignupRequest{})
	gob.Register(common.SignupResponse{})
}

// NewController creates a new instance of the controller.
func NewController(pid string, config ControllerConfig) *Controller {
	return &Controller{
		listenAddress:   config.ListenAddress,
		shutdown:        make(chan struct{}),
		done:            make(chan struct{}),
		failureDetector: newFailureDetector(pid + "-fd"),
		workerRegistry:  newWorkerRegistry(pid + ""),
		failedWorker:    make(chan failedWorkerRequest, 5),
		pid:             pid,
		clock:           clock.NewClock(pid),
		log:             clock.NewClockLog(pid, pid, clock.ClockLogConfig{Priority: config.LogPriority}),
	}
}

func (c *Controller) Start() (net.Addr, error) {

	c.log.LogDebugf(c.clock.Tick(), "Controller started failure detector")
	go c.failureDetector.Start(c.failedWorker)

	// Starts a listener
	listener, err := net.Listen("tcp", c.listenAddress)
	if err != nil {
		c.log.LogErrorf(c.clock.Tick(), "controller failed to start listener: %v", err)
		return nil, fmt.Errorf("controller failed to start listener: %v", err)
	}
	c.log.LogDebugf(c.clock.Tick(), "Controller listenting  on %s", listener.Addr().String())
	go c.handleConnections(listener)

	go func() {
		// Go routine to handle channels
		for {
			select {
			case <-c.shutdown:
				c.log.LogInfof(c.clock.Tick(), "Shutdown received. Cleaning up....")
				listener.Close()
				<-c.failureDetector.Shutdown()
				c.log.LogInfof(c.clock.Tick(), "Cleaning done")
				close(c.done)
				return
			case failedWorkerRequest := <-c.failedWorker:
				c.clock.Merge(failedWorkerRequest.Clock)
				workerId := failedWorkerRequest.workerId
				c.log.LogInfof(c.clock.Tick(), "Worker %v failed, deregistering", workerId)
				c.workerRegistry.deregisterWorker(workerId)
			}
		}
	}()

	return listener.Addr(), nil

}

func (c *Controller) handleConnections(listener net.Listener) {
	// Main loop to handle connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the error is due to listener closure
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return // Listener was closed
			}
			c.log.LogErrorf(c.clock.Tick(), "Error accepting connection: %v\n", err)
			return
		}

		go c.HandleClient(conn)
	}
}

// Shutdown gracefully shuts down the controller and worker nodes.
func (c *Controller) Shutdown() <-chan struct{} {
	c.log.LogInfof(c.clock.Tick(), "Shutdown received")
	close(c.shutdown) // Close the controller's shutdown channel
	return c.done
}

// HandleClient handles incoming client connections for the controller.
func (c *Controller) HandleClient(conn net.Conn) {
	defer conn.Close()

	// Decode the message received or fail
	var data interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&data); err != nil {
		c.log.LogErrorf(c.clock.Tick(), "Error decoding message: %v\n", err)
		return
	}

	// Switch between decoded messages
	switch mt := data.(type) {
	case common.SignupRequest:
		c.handleSignupRequest(data.(common.SignupRequest), conn)
	case common.JobSubmitRequest:
		c.handleJobSubmitRequest(data.(common.JobSubmitRequest), conn)
	default:
		c.log.LogErrorf(c.clock.Tick(), "%v message type received but not handled", mt)
	}
}

// Handles signup requests from client
func (c *Controller) handleSignupRequest(signupRequest common.SignupRequest, conn net.Conn) {

	c.log.LogInfof(c.clock.Tick(), "New signup request from %v", signupRequest.Id)

	workerInfo := workerInfo{
		id:      workerId(signupRequest.Id),
		address: workerAddress(signupRequest.Address),
	}

	if c.workerRegistry.registerWorker(workerInfo) {
		c.log.LogDebugf(c.clock.Tick(), "Worker %v added to available workers", signupRequest.Id)
	} else {
		c.log.LogDebugf(c.clock.Tick(), "Worker %v already in available workers", signupRequest.Id)
	}

	var signupResponse = common.SignupResponse{Response: common.Response{Success: true}}
	encoder := gob.NewEncoder(conn)
	var response interface{} = signupResponse
	if err := encoder.Encode(&response); err != nil {
		c.log.LogDebugf(c.clock.Tick(), "Signup error to server: %v", err)
	}

	cc := c.clock.Tick()
	c.log.LogInfof(cc, "Start failure detector request %s", signupRequest.Id)

	// monitorWorkerRequest := common.MonitorWorkerRequest{Address: signupRequest.Address}
	c.failureDetector.watchWorker <- watchWorkerRequest{
		ClockPayload: clock.ClockPayload{
			Pid:   c.pid,
			Clock: cc},
		workerInfo: workerInfo}
}

func (c *Controller) handleJobSubmitRequest(jobSubmitRequest common.JobSubmitRequest, conn net.Conn) {

}
