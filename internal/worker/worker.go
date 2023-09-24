package worker

import (
	"context"
	"encoding/gob"
	"fmt"
	"mursisoy/wordcount/internal/clock"
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
	pid               string
	clock             *clock.Clock
	log               *clock.ClockLogger
	// pendingJobs      []job
	// assignedJobs     map[worker]job
	// availableWorkers []worker
}

// WorkerConfig configures worker. See defaults in GetDefaultConfig.
type WorkerConfig struct {
	ControllerAddress string
	ListenAddress     string
	LogPriority       clock.LogPriority
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
func NewWorker(pid string, config WorkerConfig) *Worker {
	return &Worker{
		done:              make(chan struct{}),
		controllerAddress: config.ControllerAddress,
		listenAddress:     config.ListenAddress,
		pid:               pid,
		clock:             clock.NewClock(pid),
		log:               clock.NewClockLog(pid, pid, clock.ClockLogConfig{Priority: config.LogPriority}),
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

	// Main loop to handle connections
	w.log.LogInfof(w.clock.Tick(), "worker listener started: %v", listener.Addr().String())
	go w.handleConnections(listener)

	// Go routine to handle shutdowns
	go func() {
		<-ctx.Done()
		listener.Close()
		w.wg.Wait()
		w.log.LogInfof(w.clock.Tick(), "worker %s shutdown", w.pid)
		close(w.done)
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
	default:
		w.log.LogErrorf(w.clock.Tick(), "%v message type received but not handled", mt)
	}
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
