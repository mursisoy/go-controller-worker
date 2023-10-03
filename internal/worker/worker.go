package worker

import (
	"context"
	"encoding/gob"
	"fmt"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/common"
	"net"
	"sync"
	"time"
)

type taskHandler struct {
	task   common.Task
	done   chan struct{}
	cancel chan struct{}
}

// Worker represents the central component for failure detection.
type Worker struct {
	// workers          []int
	done              chan struct{}
	wg                sync.WaitGroup
	controllerAddress string
	listenAddress     string
	pid               string
	clock             *clock.Clock
	log               *clock.ClockLogger
	runningTask       *taskHandler
	taskMutex         sync.Mutex
	// pendingJobs      []job
	// assignedJobs     map[worker]job
	// availableWorkers []worker
}

// WorkerConfig configures worker. See defaults in GetDefaultConfig.
type WorkerConfig struct {
	ControllerAddress string
	ListenAddress     string
	ClockLogConfig    clock.ClockLogConfig
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
	gob.Register(common.TaskSubmitResponse{})
	gob.Register(common.TaskSubmitRequest{})
	gob.Register(common.TaskDoneRequest{})
}

// NewWorker creates a new instance of the worker.
func NewWorker(pid string, config WorkerConfig) *Worker {
	return &Worker{
		done:              make(chan struct{}),
		controllerAddress: config.ControllerAddress,
		listenAddress:     config.ListenAddress,
		pid:               pid,
		clock:             clock.NewClock(pid),
		log:               clock.NewClockLog(pid, config.ClockLogConfig),
		runningTask:       nil,
	}
}

func (w *Worker) Done() <-chan struct{} {
	return w.done
}

func (w *Worker) shutdown() {
	w.wg.Wait()
	if w.runningTask != nil {
		close(w.runningTask.cancel)
	}
	w.log.LogInfof(w.clock.Tick(), "worker %s shutdown", w.pid)
	close(w.done)
}

func (w *Worker) Start(ctx context.Context) (net.Addr, error) {
	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", w.listenAddress)
	if err != nil {
		return nil, fmt.Errorf("worker failed to start listener: %v", err)
	}
	w.listenAddress = listener.Addr().String()

	// Main loop to handle connections
	w.log.LogInfof(w.clock.Tick(), "worker listener started: %v", listener.Addr().String())
	go w.handleConnections(listener)

	// Go routine to handle shutdowns
	go func() {
		<-ctx.Done()
		listener.Close()
		w.shutdown()
	}()

	if err := w.signup(); err != nil {
		w.log.LogErrorf(w.clock.Tick(), "signup failed: %v. Exiting", err)
		return nil, fmt.Errorf("signup failed: %v. Exiting", err)
	}
	return listener.Addr(), nil
}

func (w *Worker) handleConnections(listener net.Listener) {
	defer w.wg.Done()
	w.wg.Add(1)
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the error is due to listener closure
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				w.log.LogErrorf(w.clock.Tick(), "closed network connection: %v", err)
				return
			}
			w.log.LogErrorf(w.clock.Tick(), "error accepting connection: %v", err)
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

	// Signup request start
	cc := w.clock.Tick()
	w.log.LogInfof(cc, "Send signup request to controller")
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
		w.clock.Merge(signupResponse.Clock)
		if signupResponse.Success {
			w.log.LogInfof(w.clock.Tick(), "signup on controller success")
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
		w.log.LogErrorf(w.clock.Tick(), "error decoding message: %v", err)
		return
	}

	// Switch between decoded messages
	switch mt := data.(type) {
	case common.Ping:
		w.handleHeartbeat(data.(common.Ping), conn)
	case common.TaskSubmitRequest:
		w.handleTask(data.(common.TaskSubmitRequest), conn)
	default:
		w.log.LogErrorf(w.clock.Tick(), "%v message type received but not handled", mt)
	}
}

func (w *Worker) handleTask(taskRequest common.TaskSubmitRequest, conn net.Conn) {
	w.clock.Tick()
	cc := w.clock.Merge(taskRequest.Clock)
	w.log.LogInfof(cc, "new task request from %s (%v)", taskRequest.Pid, conn.RemoteAddr())
	cc = w.clock.Tick()
	w.log.LogInfof(cc, "Send task submit response to %s (%v)", taskRequest.Pid, conn.RemoteAddr())
	var taskSubmitResponse = common.TaskSubmitResponse{
		Response: common.ResponseWithClock(w.pid, cc, false),
	}

	if err := w.runTask(taskRequest.Task); err == nil {
		taskSubmitResponse.Success = true
	}

	encoder := gob.NewEncoder(conn)
	var response interface{} = taskSubmitResponse
	if err := encoder.Encode(&response); err != nil {
		w.log.LogErrorf(w.clock.Tick(), "task submit response error to server: %v", err)
	}
}

func (w *Worker) runTask(task common.Task) error {
	defer w.taskMutex.Unlock()
	w.taskMutex.Lock()

	if w.runningTask != nil {
		return fmt.Errorf("Worker already working on job %v", w.runningTask.task.JobId)
	}

	cancel := make(chan struct{})

	w.runningTask = &taskHandler{
		task:   task,
		cancel: cancel,
	}
	go func(cancel chan struct{}) {
		var taskTicker, failTicker, crashTicker *time.Ticker
		taskTicker = time.NewTicker(task.Duration)
		if task.WillFail > 0 {
			failTicker = time.NewTicker(task.WillFail)
		} else {
			failTicker = time.NewTicker(1 * time.Second)
			failTicker.Stop()
		}
		if task.WillCrash > 0 {
			crashTicker = time.NewTicker(task.WillCrash)
		} else {
			crashTicker = time.NewTicker(1 * time.Second)
			crashTicker.Stop()
		}
	RunningTaskLoop:
		for {
			select {
			case <-cancel:
				w.log.LogInfof(w.clock.Tick(), "Task  %v canceled", task.JobId)
				break RunningTaskLoop
			case c := <-failTicker.C:
				w.log.LogInfof(w.clock.Tick(), "Task failed at: %v", c)
				break RunningTaskLoop
			case c := <-crashTicker.C:
				w.log.LogInfof(w.clock.Tick(), "Task failed at: %v", c)
				break RunningTaskLoop
			case c := <-taskTicker.C:
				task.Result = c
				w.log.LogInfof(w.clock.Tick(), "Task done, result: %v", c)

				conn, err := net.Dial("tcp", string(w.controllerAddress))
				if err != nil {
					w.log.LogErrorf(w.clock.Tick(), "Fail to send done task to controller (%v)", w.controllerAddress)
					break
				}

				cc := w.clock.Tick()
				w.log.LogInfof(cc, "Send done task to controller (%v)", w.controllerAddress)
				var taskDoneRequest = common.TaskDoneRequest{
					Request: common.RequestWithClock(w.pid, cc),
					Task:    task,
				}
				encoder := gob.NewEncoder(conn)
				var response interface{} = taskDoneRequest
				if err := encoder.Encode(&response); err != nil {
					w.log.LogErrorf(w.clock.Tick(), "Sen donde task error to controller: %v", err)
				}
				break RunningTaskLoop
			}
		}
		w.runningTask = nil
		if taskTicker != nil {
			taskTicker.Stop()
		}
		if failTicker != nil {
			failTicker.Stop()
		}
		if crashTicker != nil {
			crashTicker.Stop()
		}
	}(cancel)
	return nil
}

func (w *Worker) handleHeartbeat(pingRequest common.Ping, conn net.Conn) {
	w.clock.Tick()
	cc := w.clock.Merge(pingRequest.Clock)
	w.log.LogInfof(cc, "new ping request from %s (%v)", pingRequest.Pid, conn.RemoteAddr())

	cc = w.clock.Tick()
	w.log.LogInfof(cc, "send pong to %s (%v)", pingRequest.Pid, conn.RemoteAddr())
	var pongResponse = common.Pong{
		ClockPayload: clock.ClockPayload{Clock: cc, Pid: w.pid},
	}
	encoder := gob.NewEncoder(conn)
	var response interface{} = pongResponse
	if err := encoder.Encode(&response); err != nil {
		w.log.LogErrorf(w.clock.Tick(), "pong error to server: %v", err)
	}
}
