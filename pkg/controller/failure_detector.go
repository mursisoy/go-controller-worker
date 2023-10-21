package controller

import (
	"encoding/gob"
	"net"
	"time"

	"github.com/mursisoy/go-controller-worker/pkg/clock"
	"github.com/mursisoy/go-controller-worker/pkg/common"
)

type watchWorkerRequest struct {
	clock.ClockPayload
	workerInfo workerInfo
}

type failedWorkerRequest struct {
	clock.ClockPayload
	workerId workerId
}

// WorkerConfig configures worker. See defaults in GetDefaultConfig.
type FailureDetectorConfig struct {
	HeartBeatTicker time.Duration
	ClockLogConfig  clock.ClockLogConfig
}

type failureDetector struct {
	pid             string
	done            chan struct{}
	shutdown        chan struct{}
	watchWorker     chan watchWorkerRequest
	unwatchWorker   chan workerId
	failedWorker    chan<- failedWorkerRequest
	watchedWorkers  map[workerId]chan struct{}
	clog            *clock.ClockLogger
	heartBeatTicker time.Duration
}

func init() {
	gob.Register(common.Ping{})
	gob.Register(common.Pong{})
}

func newFailureDetector(pid string, config FailureDetectorConfig) *failureDetector {
	return &failureDetector{
		pid:             pid,
		done:            make(chan struct{}),
		shutdown:        make(chan struct{}),
		watchWorker:     make(chan watchWorkerRequest, 1),
		unwatchWorker:   make(chan workerId, 1),
		watchedWorkers:  make(map[workerId]chan struct{}),
		clog:            clock.NewClockLog(pid, config.ClockLogConfig),
		heartBeatTicker: config.HeartBeatTicker,
	}
}

func (fd *failureDetector) Shutdown() <-chan struct{} {
	fd.clog.LogInfof("Failure detector shutdown received")
	close(fd.shutdown)
	return fd.done
}

func (fd *failureDetector) Start(failedWorker chan<- failedWorkerRequest) {
	fd.clog.LogInfof("Failure detector started")
	fd.failedWorker = failedWorker
	for {
		select {
		case <-fd.shutdown:
			fd.clog.LogInfof("Failure detector shutdown. Cleaning up")
			for workerId, done := range fd.watchedWorkers {
				fd.clog.LogInfof("Stopping failure detector for worker %v", workerId)
				close(done)
			}
			fd.clog.LogInfof("Failure detector shutdown. Cleaning done")
			fd.clog.Close()
			close(fd.done)
			return

		case watchWorkerRequest := <-fd.watchWorker:
			workerInfo := watchWorkerRequest.workerInfo
			fd.clog.LogMergeInfof(watchWorkerRequest.Clock, "Watching worker %v", workerInfo.id)
			if _, ok := fd.watchedWorkers[workerInfo.id]; ok {
				close(fd.watchedWorkers[workerInfo.id])
			}

			done := make(chan struct{})
			fd.watchedWorkers[workerInfo.id] = done
			go fd.monitorWorker(workerInfo, done)

		case workerId := <-fd.unwatchWorker:
			fd.clog.LogInfof("Un-watching worker %v", workerId)
			if _, ok := fd.watchedWorkers[workerId]; ok {
				fd.clog.LogInfof("Stopping failure detector for worker %v", workerId)
				close(fd.watchedWorkers[workerId])
				delete(fd.watchedWorkers, workerId)
			} else {
				fd.clog.LogInfof("Worker was not under observation %v", workerId)
			}
		}

	}
}

func (fd *failureDetector) monitorWorker(workerInfo workerInfo, done chan struct{}) {
	ticker := time.NewTicker(fd.heartBeatTicker)
	for {
		select {
		case <-done:
			ticker.Stop()
			fd.clog.LogInfof("Failure detector for worker %v stopped", workerInfo.id)
			return
		case t := <-ticker.C:
			fd.clog.LogInfof("Sending heartbeat to %v at %v", workerInfo.id, t)
			if ok := fd.heartbeat(workerInfo); !ok {
				cc := fd.clog.LogErrorf("Heartbeat to %v at %v failed", workerInfo.id, t)
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
		fd.clog.LogInfof("Error connecting to worker: %v", err)
		return false
	}

	cc := fd.clog.LogInfof("New ping request to %s (%v)", workerInfo.id, workerInfo.address)

	pingRequest := common.Ping{ClockPayload: clock.ClockPayload{Clock: cc, Pid: fd.pid}}
	var request interface{} = pingRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		fd.clog.LogInfof("Ping error to worker: %v", err)
		return false
	}

	conn.SetDeadline(time.Now().Add(1 * time.Second))
	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		fd.clog.LogInfof("Error decoding message: %v", err)
		return false
	}
	switch mt := response.(type) {
	case common.Pong:
		fd.clog.LogMergeInfof(mt.Clock, "Pong received from %v (%v)", workerInfo.id, conn.RemoteAddr().String())
		return true
	default:
		fd.clog.LogInfof("Pong error, received message type %v from %v(%v): %v", mt, workerInfo.id, conn.RemoteAddr().String(), response)
		return false
	}
}
