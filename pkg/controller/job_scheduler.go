package controller

// import (
// 	"github.com/mursisoy/go-controller-worker/pkg/clock"
// 	"github.com/mursisoy/go-controller-worker/pkg/common"
// 	"sync/atomic"
// )

// type jobHandle struct {
// 	job            common.Job
// 	assignedWorker workerInfo
// 	done           chan struct{}
// 	cancel         chan struct{}
// }

// type jobStack []*jobHandle

// func (q *jobStack) push(n *jobHandle) {
// 	*q = append(*q, n)
// }

// func (q *jobStack) pop() (n *jobHandle) {
// 	x := q.Len() - 1
// 	n = (*q)[x]
// 	*q = (*q)[:x]
// 	return
// }
// func (q *jobStack) Len() int {
// 	return len(*q)
// }

// type JobScheduler struct {
// 	jobCount    uint32
// 	clock       *clock.Clock
// 	log         *clock.ClockLogger
// 	jobQueue    jobStack
// 	runningJobs map[common.JobId]*jobHandle
// 	enqueueJob  chan *common.Job
// 	shutdown    chan struct{}
// 	done        chan struct{}
// 	jobs        chan common.JobSubmitRequest
// }

// func newJobScheduler(pid string) *JobScheduler {
// 	return &JobScheduler{
// 		jobCount: 0,
// 		clock:    clock.NewClock(pid),
// 		log:      clock.NewClockLog(pid, clock.ClockLogConfig{}),
// 		jobQueue: make(jobStack, 0),
// 		jobs:     make(chan common.JobSubmitRequest, 1),
// 		shutdown: make(chan struct{}),
// 		done:     make(chan struct{}),
// 	}
// }

// func (js *JobScheduler) Start() {
// 	go func() {
// 		for {
// 			select {
// 			case <-js.shutdown:
// 				js.log.LogInfof(js.clock.Tick(), "Shutdown received. Cleaning up....")
// 				for jobId, jobHandle := range js.runningJobs {
// 					js.log.LogInfof(js.clock.Tick(), "Stopping job handle %v", jobId)
// 					close(jobHandle.cancel)
// 				}
// 				js.log.LogInfof(js.clock.Tick(), "Job scheduler cleaning done")
// 				close(js.done)
// 				return
// 			case job := <-js.jobs:
// 				js.scheduleJob(job)
// 			}
// 		}
// 	}()
// }

// func (js *JobScheduler) scheduleJob(jsr common.JobSubmitRequest) {
// 	js.clock.Tick()
// 	cc := js.clock.Merge(jsr.Clock)
// 	js.log.LogInfof(cc "Scheduling job")
// 	jobId := atomic.AddUint32(&js.jobCount, 1)
// 	jsr.Job.Id = common.JobId(jobId)

// }

// func (js *JobScheduler) Shutdown() <-chan struct{} {
// 	js.log.LogInfof(js.clock.Tick(), "Failure detector shutdown received")
// 	close(js.shutdown)
// 	return js.done
// }

// func (js *JobScheduler) SubmitJob() chan<- common.JobSubmitRequest {
// 	return js.jobs
// }

// func (js *JobScheduler) cancelJob() {

// }
