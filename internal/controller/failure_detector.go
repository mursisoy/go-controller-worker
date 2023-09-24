package controller

import (
	"encoding/gob"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/common"
	"net"
	"time"
)

type watchWorkerRequest struct {
	clock.ClockPayload
	workerInfo workerInfo
}

type failedWorkerRequest struct {
	clock.ClockPayload
	workerId workerId
}

type failureDetector struct {
	pid            string
	done           chan struct{}
	shutdown       chan struct{}
	watchWorker    chan watchWorkerRequest
	unwatchWorker  chan workerId
	failedWorker   chan<- failedWorkerRequest
	watchedWorkers map[workerId]chan struct{}
	clock          *clock.Clock
	log            *clock.ClockLogger
}

func init() {
	gob.Register(common.Ping{})
	gob.Register(common.Pong{})
}

func newFailureDetector(pid string) *failureDetector {
	return &failureDetector{
		pid:            pid,
		done:           make(chan struct{}),
		shutdown:       make(chan struct{}),
		watchWorker:    make(chan watchWorkerRequest, 1),
		unwatchWorker:  make(chan workerId, 1),
		watchedWorkers: make(map[workerId]chan struct{}),
		clock:          clock.NewClock(pid),
		log:            clock.NewClockLog(pid, pid, clock.ClockLogConfig{}),
	}
}

func (fd *failureDetector) Shutdown() <-chan struct{} {
	fd.log.LogInfof(fd.clock.Tick(), "Failure detector shutdown received")
	close(fd.shutdown)
	return fd.done
}

func (fd *failureDetector) Start(failedWorker chan<- failedWorkerRequest) {
	fd.log.LogInfof(fd.clock.Tick(), "Failure detector started")
	fd.failedWorker = failedWorker
	for {
		select {
		case <-fd.shutdown:
			fd.log.LogInfof(fd.clock.Tick(), "Failure detector shutdown. Cleaning up")
			for workerId, done := range fd.watchedWorkers {
				fd.log.LogInfof(fd.clock.Tick(), "Stopping failure detector for worker %v", workerId)
				close(done)
			}
			fd.log.LogInfof(fd.clock.Tick(), "Failure detector shutdown. Cleaning done")
			fd.log.Close()
			close(fd.done)
			return

		case watchWorkerRequest := <-fd.watchWorker:
			fd.clock.Tick()
			cc := fd.clock.Merge(watchWorkerRequest.Clock)
			workerInfo := watchWorkerRequest.workerInfo
			if _, ok := fd.watchedWorkers[workerInfo.id]; ok {
				close(fd.watchedWorkers[workerInfo.id])
			}
			fd.log.LogInfof(cc, "Watching worker %v", workerInfo.id)
			done := make(chan struct{})
			fd.watchedWorkers[workerInfo.id] = done
			go fd.monitorWorker(workerInfo, done)

		case workerId := <-fd.unwatchWorker:
			fd.log.LogInfof(fd.clock.Tick(), "Un-watching worker %v", workerId)
			if _, ok := fd.watchedWorkers[workerId]; ok {
				fd.log.LogInfof(fd.clock.Tick(), "Stopping failure detector for worker %v", workerId)
				close(fd.watchedWorkers[workerId])
				delete(fd.watchedWorkers, workerId)
			} else {
				fd.log.LogInfof(fd.clock.Tick(), "Worker was not under observation %v", workerId)
			}
		}

	}
}

func (fd *failureDetector) monitorWorker(workerInfo workerInfo, done chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-done:
			ticker.Stop()
			fd.log.LogInfof(fd.clock.Tick(), "Failure detector for worker %v stopped", workerInfo.id)
			return
		case t := <-ticker.C:
			fd.log.LogInfof(fd.clock.Tick(), "Sending heartbeat to %v at %v", workerInfo.id, t)
			if ok := fd.heartbeat(workerInfo); !ok {
				cc := fd.clock.Tick()
				fd.log.LogErrorf(cc, "Heartbeat to %v at %v failed", workerInfo.id, t)
				ticker.Stop()
				fd.unwatchWorker <- workerInfo.id
				fd.failedWorker <- failedWorkerRequest{
					ClockPayload: clock.ClockPayload{
						Clock: cc,
						Pid:   fd.pid,
					},
					workerId: workerInfo.id,
				}
			}
		}
	}
}

func (fd *failureDetector) heartbeat(workerInfo workerInfo) bool {
	conn, err := net.Dial("tcp", string(workerInfo.address))
	if err != nil {
		fd.log.LogInfof(fd.clock.Tick(), "Error connecting to worker: %v", err)
		return false
	}

	cc := fd.clock.Tick()
	fd.log.LogInfof(cc, "New ping request to %s (%v)", workerInfo.id, workerInfo.address)

	pingRequest := common.Ping{ClockPayload: clock.ClockPayload{Clock: cc, Pid: fd.pid}}
	var request interface{} = pingRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		fd.log.LogInfof(fd.clock.Tick(), "Ping error to worker: %v", err)
		return false
	}

	conn.SetDeadline(time.Now().Add(1 * time.Second))
	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		fd.log.LogInfof(fd.clock.Tick(), "Error decoding message: %v", err)
		return false
	}
	cc = fd.clock.Tick()
	switch mt := response.(type) {
	case common.Pong:
		cc = fd.clock.Merge(mt.Clock)
		fd.log.LogInfof(cc, "Pong received from %v (%v)", workerInfo.id, conn.RemoteAddr().String())
		return true
	default:
		fd.log.LogInfof(cc, "Pong error, received message type %v from %v(%v): %v", mt, workerInfo.id, conn.RemoteAddr().String(), response)
		return false
	}
}
