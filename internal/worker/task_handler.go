package worker

import (
	"encoding/gob"
	"fmt"
	"mursisoy/wordcount/internal/common"
	"net"
	"time"
)

func init() {
	gob.Register(common.TaskSubmitResponse{})
	gob.Register(common.TaskSubmitRequest{})
	gob.Register(common.TaskDoneRequest{})
}

func (w *Worker) handleTask(taskRequest common.TaskSubmitRequest, conn net.Conn) {

	w.clog.LogMergeInfof(taskRequest.Clock, "new task request from %s (%v)", taskRequest.Pid, conn.RemoteAddr())
	cc := w.clog.LogInfof("Send task submit response to %s (%v)", taskRequest.Pid, conn.RemoteAddr())
	var taskSubmitResponse = common.TaskSubmitResponse{
		Response: common.ResponseWithClock(w.pid, cc, false),
	}

	if err := w.runTask(taskRequest.Task); err == nil {
		taskSubmitResponse.Success = true
	}

	encoder := gob.NewEncoder(conn)
	var response interface{} = taskSubmitResponse
	if err := encoder.Encode(&response); err != nil {
		w.clog.LogErrorf("task submit response error to server: %v", err)
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
				w.clog.LogInfof("Task  %v canceled", task.JobId)
				break RunningTaskLoop
			case c := <-failTicker.C:
				w.clog.LogInfof("Task failed at: %v", c)
				break RunningTaskLoop
			case c := <-crashTicker.C:
				w.clog.LogInfof("Task failed at: %v", c)
				break RunningTaskLoop
			case c := <-taskTicker.C:
				task.Result = c
				w.clog.LogInfof("Task done, result: %v", c)

				conn, err := net.Dial("tcp", string(w.controllerAddress))
				if err != nil {
					w.clog.LogErrorf("Fail to send done task to controller (%v)", w.controllerAddress)
					break
				}

				cc := w.clog.LogInfof("Send done task to controller (%v)", w.controllerAddress)
				var taskDoneRequest = common.TaskDoneRequest{
					Request: common.RequestWithClock(w.pid, cc),
					Task:    task,
				}
				encoder := gob.NewEncoder(conn)
				var response interface{} = taskDoneRequest
				if err := encoder.Encode(&response); err != nil {
					w.clog.LogErrorf("Sen donde task error to controller: %v", err)
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
