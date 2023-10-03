// White-box testing
package controller

import (
	"context"
	"encoding/gob"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/common"
	"mursisoy/wordcount/internal/worker"
	"net"
	"testing"
	"time"
)

// Be sure to update your imports

// // Below init function
// func TestController(t *testing.T) {
// 	controller := NewController()
// 	controller.Start()
// 	defer controller.Shutdown()

// 	// Simply check that the server is up and can
// 	// accept connections.
// 	conn, err := net.Dial("tcp", ":1123")
// 	if err != nil {
// 		t.Error("could not connect to server: ", err)
// 	}
// 	defer conn.Close()
// }

func newController() *Controller {
	controllerConfig := ControllerConfig{
		ListenAddress: ":0",
		LogPriority:   clock.DEBUG,
		FailureDetectorConfig: FailureDetectorConfig{
			HeartBeatTicker: 2 * time.Second,
		},
	}
	return NewController("test-controller", controllerConfig)
}

func newWorker(workerId string, controllerAddress string) *worker.Worker {
	workerConfig := worker.WorkerConfig{
		ControllerAddress: controllerAddress,
		ListenAddress:     ":0",
	}
	// Notify the sigCh channel for SIGINT (Ctrl+C) and SIGTERM (termination) signals.
	return worker.NewWorker(workerId, workerConfig)
}

type workerData struct {
	worker *worker.Worker
	ctx    context.Context
	cancel context.CancelFunc
}

func TestRegisteredAndFailedWorkers(t *testing.T) {
	// var workerList map[workerId]workerData
	// var wg sync.WaitGroup
	var controllerAddress net.Addr
	var err error

	// Starts two workers, send a job to one of them and let it finish
	controller := newController()
	if controllerAddress, err = controller.Start(); err != nil {
		t.Fatalf("Error starting controller: %v", err)
	}

	// workerList = make(map[workerId]workerData, 2)
	w1 := workerData{
		worker: newWorker("w1", controllerAddress.String()),
	}
	w1.ctx, w1.cancel = context.WithCancel(context.Background())

	w2 := workerData{
		worker: newWorker("w2", controllerAddress.String()),
	}
	w2.ctx, w2.cancel = context.WithCancel(context.Background())

	w1.worker.Start(w1.ctx)
	time.Sleep(1 * time.Second)
	if !controller.workerRegistry.isRegistered("w1") {
		t.Fatalf("Worker w1 not registered in controller")
	}
	go func() {
		time.Sleep(2 * time.Second)
		w1.cancel()
		<-w1.ctx.Done()
	}()
	w2.worker.Start(w2.ctx)

	// t.Logf("Wait ten seconds")
	time.Sleep(6 * time.Second)

	if controller.workerRegistry.isRegistered("w1") {
		t.Fatalf("Worker w1 is, should have failed")
	}
	if !controller.workerRegistry.isRegistered("w2") {
		t.Fatalf("Worker w2 should stay registered")
	}

	<-controller.Shutdown()
	w2.cancel()
	<-w2.ctx.Done()
	<-w2.worker.Done()
	<-w1.worker.Done()

	t.Logf("Shutdown completed")
}

func TestCorrectJobSubmission(t *testing.T) {
	var controllerAddress net.Addr
	var err error
	pid := "t"
	testClock := clock.NewClock(pid)
	testClockLog := clock.NewClockLog(pid, clock.ClockLogConfig{})
	// Starts two workers, send a job to one of them and let it finish
	controller := newController()
	if controllerAddress, err = controller.Start(); err != nil {
		t.Fatalf("Error starting controller: %v", err)
	}

	// workerList = make(map[workerId]workerData, 2)
	w1 := workerData{
		worker: newWorker("w1", controllerAddress.String()),
	}
	w1.ctx, w1.cancel = context.WithCancel(context.Background())
	w1.worker.Start(w1.ctx)

	conn, err := net.Dial("tcp", controllerAddress.String())
	if err != nil {
		t.Fatalf("Error connecting to worker: %v", err)
	}

	cc := testClock.Tick()
	testClockLog.LogDebugf(cc, "Send job to controller ")
	jobSubmitRequest := common.JobSubmitRequest{
		Request: common.RequestWithClock(pid, cc),
		Job: common.Job{
			Duration: 3 * time.Second,
		},
	}

	var request interface{} = jobSubmitRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		t.Fatalf("Ping error to worker: %v", err)
	}

	conn.SetDeadline(time.Now().Add(1 * time.Second))
	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Error decoding message: %v", err)
	}

	testClock.Tick()
	switch mt := response.(type) {
	case common.JobSubmitResponse:
		cc = testClock.Merge(mt.Clock)
		if mt.Response.Success {
			testClockLog.LogInfof(cc, "Received success job response with job id %v", mt.Job.Id)
		} else {
			t.Fatalf("Received unsucessful response from controller")
		}
	default:
		t.Fatalf("Received  unknown response from controller")
	}

	time.Sleep(5 * time.Second)
}

func TestRetryOnWorkerFailedJobSubmission(t *testing.T) {
	var controllerAddress net.Addr
	var err error
	pid := "t"
	testClock := clock.NewClock(pid)
	testClockLog := clock.NewClockLog(pid, clock.ClockLogConfig{})
	// Starts two workers, send a job to one of them and let it finish
	controller := newController()
	if controllerAddress, err = controller.Start(); err != nil {
		t.Fatalf("Error starting controller: %v", err)
	}

	// workerList = make(map[workerId]workerData, 2)
	w1 := workerData{
		worker: newWorker("w1", controllerAddress.String()),
	}
	w1.ctx, w1.cancel = context.WithCancel(context.Background())

	w2 := workerData{
		worker: newWorker("w2", controllerAddress.String()),
	}
	w2.ctx, w2.cancel = context.WithCancel(context.Background())

	w1.worker.Start(w1.ctx)
	time.Sleep(1 * time.Second)
	if !controller.workerRegistry.isRegistered("w1") {
		t.Fatalf("Worker w1 not registered in controller")
	}

	w2.worker.Start(w2.ctx)
	time.Sleep(1 * time.Second)
	if !controller.workerRegistry.isRegistered("w2") {
		t.Fatalf("Worker w2 not registered in controller")
	}

	conn, err := net.Dial("tcp", controllerAddress.String())
	if err != nil {
		t.Fatalf("Error connecting to worker: %v", err)
	}
	cc := testClock.Tick()
	testClockLog.LogDebugf(cc, "Send job to controller ")
	jobSubmitRequest := common.JobSubmitRequest{
		Request: common.RequestWithClock(pid, cc),
		Job: common.Job{
			Duration: 3 * time.Second,
		},
	}

	var request interface{} = jobSubmitRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		t.Fatalf("Ping error to worker: %v", err)
	}

	conn.SetDeadline(time.Now().Add(1 * time.Second))
	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Error decoding message: %v", err)
	}

	testClock.Tick()
	switch mt := response.(type) {
	case common.JobSubmitResponse:
		cc = testClock.Merge(mt.Clock)
		if mt.Response.Success {
			testClockLog.LogInfof(cc, "Received success job response with job id %v", mt.Job.Id)
		} else {
			t.Fatalf("Received unsucessful response from controller")
		}
	default:
		t.Fatalf("Received  unknown response from controller")
	}

	go func() {
		time.Sleep(1 * time.Second)
		w1.cancel()
		<-w1.ctx.Done()
	}()

	time.Sleep(10 * time.Second)
}
