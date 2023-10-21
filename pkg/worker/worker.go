package worker

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/mursisoy/go-controller-worker/pkg/clock"
	"github.com/mursisoy/go-controller-worker/pkg/common"
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
	clog              *clock.ClockLogger
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

type messageHandlerCallback func(message interface{}, conn net.Conn)

// var handleMessageType := {
// 	common.Ping:
// }

// NewWorker creates a new instance of the worker.
func NewWorker(pid string, config WorkerConfig) *Worker {
	return &Worker{
		done:              make(chan struct{}),
		controllerAddress: config.ControllerAddress,
		listenAddress:     config.ListenAddress,
		pid:               pid,
		clog:              clock.NewClockLog(pid, config.ClockLogConfig),
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
	w.clog.LogInfof("worker %s shutdown", w.pid)
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
	w.clog.LogInfof("worker listener started: %v", listener.Addr().String())
	go func() {
		w.wg.Add(1)
		defer w.wg.Done()
		common.HandleConnections(listener, w.handleClient)
	}()

	// Go routine to handle shutdowns
	go func() {
		<-ctx.Done()
		listener.Close()
		w.shutdown()
	}()

	if err := w.signup(); err != nil {
		w.clog.LogErrorf("signup failed: %v. Exiting", err)
		return nil, fmt.Errorf("signup failed: %v. Exiting", err)
	}
	return listener.Addr(), nil
}

// HandleClient handles incoming client connections for the controller.
func (w *Worker) handleClient(conn net.Conn) {
	w.wg.Add(1)
	defer w.wg.Done()
	defer conn.Close()

	// Decode the message received or fail
	data, err := common.DecodeMessage(conn)
	if err != nil {
		w.clog.LogErrorf("Error decoding message: %v", err)
		return
	}

	// Switch between decoded messages
	switch mt := data.(type) {
	case common.Ping:
		w.handleHeartbeat(data.(common.Ping), conn)
	case common.TaskSubmitRequest:
		w.handleTask(data.(common.TaskSubmitRequest), conn)
	default:
		w.clog.LogErrorf("%v message type received but not handled", mt)
	}
}
