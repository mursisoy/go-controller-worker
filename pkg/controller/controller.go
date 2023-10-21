package controller

import (
	"encoding/gob"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/mursisoy/go-controller-worker/pkg/clock"
	"github.com/mursisoy/go-controller-worker/pkg/common"
)

type jobHandler struct {
	job            common.Job
	assignedWorker *workerInfo
	done           chan struct{}
	cancel         chan struct{}
}

// WorkerConfig configures worker. See defaults in GetDefaultConfig.
type ControllerConfig struct {
	ListenAddress         string
	LogPriority           clock.LogPriority
	ClockLogConfig        clock.ClockLogConfig
	FailureDetectorConfig FailureDetectorConfig
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
	log             *clock.ClockLogger
	jobCount        uint32
	jobRegistry     map[common.JobId]jobHandler
	jobQueue        chan jobHandler
}

func init() {
	gob.Register(common.SignupRequest{})
	gob.Register(common.SignupResponse{})
	gob.Register(common.JobSubmitResponse{})
	gob.Register(common.JobSubmitRequest{})
	gob.Register(common.TaskSubmitResponse{})
	gob.Register(common.TaskSubmitRequest{})
	gob.Register(common.TaskDoneRequest{})
}

// NewController creates a new instance of the controller.
func NewController(pid string, config ControllerConfig) *Controller {
	return &Controller{
		listenAddress:   config.ListenAddress,
		shutdown:        make(chan struct{}),
		done:            make(chan struct{}),
		failureDetector: newFailureDetector(pid+"-fd", config.FailureDetectorConfig),
		workerRegistry:  newWorkerRegistry(),
		failedWorker:    make(chan failedWorkerRequest, 5),
		jobQueue:        make(chan jobHandler, 100),
		pid:             pid,
		log:             clock.NewClockLog(pid, config.ClockLogConfig),
		jobRegistry:     make(map[common.JobId]jobHandler),
	}
}

const maxJobRetries = 3

func (c *Controller) Start() (net.Addr, error) {

	// Starts a listener
	listener, err := net.Listen("tcp", c.listenAddress)
	if err != nil {
		c.log.LogErrorf("controller failed to start listener: %v", err)
		return nil, fmt.Errorf("controller failed to start listener: %v", err)
	}

	c.log.LogDebugf("Controller listenting  on %s", listener.Addr().String())
	go c.handleConnections(listener)

	c.log.LogDebugf("Controller started failure detector")
	go c.failureDetector.Start(c.failedWorker)

	c.log.LogDebugf("Start job scheduler")
	go c.scheduleJobs()

	go func() {
		// Go routine to handle channels
		for {
			select {
			case <-c.shutdown:
				c.log.LogInfof("Shutdown received. Cleaning up....")
				c.log.LogInfof("Close job queue")
				close(c.jobQueue)
				listener.Close()
				<-c.failureDetector.Shutdown()
				c.log.LogInfof("Cleaning done")
				close(c.done)
				return
			case failedWorkerRequest := <-c.failedWorker:
				workerId := failedWorkerRequest.workerId
				c.log.LogMergeInfof(failedWorkerRequest.Clock, "Worker %v failed, deregistering", workerId)
				c.workerRegistry.deregisterWorker(workerId)
			}
		}
	}()

	c.listenAddress = listener.Addr().String()
	return listener.Addr(), nil
}

func (c *Controller) scheduleJobs() {
	for jobHandler := range c.jobQueue {
		jobRetries := 0
	TaskSubmitLoop:
		for {

			if jobRetries > maxJobRetries {
				break TaskSubmitLoop
			} else {
				jobRetries = jobRetries + 1
			}

			<-c.workerRegistry.noAvailableWorkers
			workerInfo, err := c.workerRegistry.getAvailableWorker()

			if err != nil {
				c.log.LogInfof("Error processing job, retrying: %v", err)
				continue
			}

			conn, err := net.Dial("tcp", string(workerInfo.address))
			if err != nil {
				c.log.LogInfof("Error connecting to worker: %v", err)
				c.failureDetector.unwatchWorker <- workerInfo.id
				c.workerRegistry.deregisterWorker(workerInfo.id)
				continue
			}

			cc := c.log.LogInfof("New task request to %s (%v)", workerInfo.id, workerInfo.address)

			taskSubmitRequest := common.TaskSubmitRequest{
				Request: common.RequestWithClock(c.pid, cc),
				Task: common.Task{
					JobId:    jobHandler.job.Id,
					Duration: jobHandler.job.Duration,
				},
			}
			var request interface{} = taskSubmitRequest
			encoder := gob.NewEncoder(conn)
			if err = encoder.Encode(&request); err != nil {
				c.log.LogInfof("Task submit error to worker: %v", err)
				c.workerRegistry.putAvailableWorker(workerInfo.id)
				continue
			}

			conn.SetDeadline(time.Now().Add(1 * time.Second))
			var response interface{}
			decoder := gob.NewDecoder(conn)
			if err := decoder.Decode(&response); err != nil {
				c.log.LogInfof("Error decoding message: %v", err)
				c.workerRegistry.putAvailableWorker(workerInfo.id)
				continue
			}

			switch mt := response.(type) {
			case common.TaskSubmitResponse:
				c.log.LogMergeInfof(mt.Clock, "Task submit success from %v (%v)", workerInfo.id, conn.RemoteAddr().String())
				jobHandler.assignedWorker = &workerInfo
				jobHandler.cancel = make(chan struct{})
				jobHandler.done = make(chan struct{})
				c.jobRegistry[jobHandler.job.Id] = jobHandler
				go c.taskWatchdog(&jobHandler)
				break TaskSubmitLoop
			default:
				c.log.LogInfof("Task submit error, received message type %v from %v(%v): %v", mt, workerInfo.id, conn.RemoteAddr().String(), response)
				c.workerRegistry.putAvailableWorker(workerInfo.id)
				continue
			}
		}

	}
}

func (c *Controller) taskWatchdog(jobHandler *jobHandler) {
	ticker := time.NewTicker(jobHandler.job.Duration + 2*time.Second)
	for {
		select {
		case <-ticker.C:
			c.log.LogInfof("JobId %v execution timeout", jobHandler.job.Id)
			jobHandler.assignedWorker = nil
			jobHandler.cancel = nil
			jobHandler.done = nil
			c.jobQueue <- *jobHandler
			return
		case <-jobHandler.done:
			c.log.LogInfof("JobId %v done", jobHandler.job.Id)
			c.workerRegistry.putAvailableWorker(jobHandler.assignedWorker.id)
			jobHandler.assignedWorker = nil
			jobHandler.cancel = nil
			jobHandler.done = nil
			return
		case <-jobHandler.cancel:
			c.log.LogInfof("JobId %v canceled", jobHandler.job.Id)
			jobHandler.assignedWorker = nil
			jobHandler.cancel = nil
			jobHandler.done = nil
			return
		}
	}
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
			c.log.LogErrorf("Error accepting connection: %v\n", err)
			return
		}

		go c.HandleClient(conn)
	}
}

// Shutdown gracefully shuts down the controller and worker nodes.
func (c *Controller) Shutdown() <-chan struct{} {
	c.log.LogInfof("Shutdown received")
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
		c.log.LogErrorf("Error decoding message: %v\n", err)
		return
	}

	// Switch between decoded messages
	switch mt := data.(type) {
	case common.SignupRequest:
		c.handleSignupRequest(data.(common.SignupRequest), conn)
	case common.JobSubmitRequest:
		c.handleJobSubmitRequest(data.(common.JobSubmitRequest), conn)
	case common.TaskDoneRequest:
		c.handleTaskDoneRequest(data.(common.TaskDoneRequest), conn)
	default:
		c.log.LogErrorf("%v message type received but not handled", mt)
	}
}

// Handles signup requests from client
func (c *Controller) handleSignupRequest(signupRequest common.SignupRequest, conn net.Conn) {

	c.log.LogMergeInfof(signupRequest.Clock, "New signup request from %v", signupRequest.Pid)

	workerInfo := workerInfo{
		id:      workerId(signupRequest.Id),
		address: workerAddress(signupRequest.Address),
	}

	if c.workerRegistry.registerWorker(workerInfo) {
		c.log.LogDebugf("Worker %v added to available workers", signupRequest.Id)
	} else {
		c.log.LogDebugf("Worker %v already in available workers", signupRequest.Id)
	}

	var signupResponse = common.SignupResponse{Response: common.Response{Success: true}}
	encoder := gob.NewEncoder(conn)
	var response interface{} = signupResponse
	if err := encoder.Encode(&response); err != nil {
		c.log.LogDebugf("Signup error to server: %v", err)
	}

	cc := c.log.LogInfof("Start failure detector request %s", signupRequest.Id)

	// monitorWorkerRequest := common.MonitorWorkerRequest{Address: signupRequest.Address}
	c.failureDetector.watchWorker <- watchWorkerRequest{
		ClockPayload: clock.ClockPayload{
			Pid:   c.pid,
			Clock: cc},
		workerInfo: workerInfo}
}

// Handles signup requests from client
func (c *Controller) handleTaskDoneRequest(taskDoneRequest common.TaskDoneRequest, conn net.Conn) {

	c.log.LogMergeInfof(taskDoneRequest.Clock, "Received task done from %v", taskDoneRequest.Pid)
	if c.jobRegistry[taskDoneRequest.Task.JobId].assignedWorker.id == workerId(taskDoneRequest.Pid) {
		close(c.jobRegistry[taskDoneRequest.Task.JobId].done)
	} else {
		c.log.LogInfof("The received task was not assigned to %v", taskDoneRequest.Pid)
		c.workerRegistry.putAvailableWorker(workerId(taskDoneRequest.Pid))
	}

}

func (c *Controller) handleJobSubmitRequest(jobSubmitRequest common.JobSubmitRequest, conn net.Conn) {
	c.log.LogMergeInfof(jobSubmitRequest.Clock, "New job request from %v", jobSubmitRequest.Pid)
	jobId := atomic.AddUint32(&c.jobCount, 1)
	jobSubmitRequest.Job.Id = common.JobId(jobId)

	jobHandle := jobHandler{
		job: jobSubmitRequest.Job,
	}

	var success bool
	select {
	case c.jobQueue <- jobHandle:
		c.log.LogDebugf("Job %v enqueued", jobHandle.job.Id)
		success = true
	default:
		c.log.LogDebugf("Job %v not queued. Full queue.", jobHandle.job.Id)
		success = false
		break
	}

	cc := c.log.LogDebugf("Send job response with id  %v.", jobHandle.job.Id)
	jobSubmitResponse := common.JobSubmitResponse{
		Response: common.ResponseWithClock(c.pid, cc, success),
		Job:      jobSubmitRequest.Job,
	}

	encoder := gob.NewEncoder(conn)
	var response interface{} = jobSubmitResponse
	if err := encoder.Encode(&response); err != nil {
		c.log.LogDebugf("Failed response to client: %v", err)
	}

}
