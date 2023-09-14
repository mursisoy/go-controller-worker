package controller

import (
	"encoding/gob"
	"log"
	"mursisoy/wordcount/internal/common"
	"net"
	"time"
)

type failureDetector struct {
	shutdown       chan struct{}
	watchWorker    chan workerInfo
	unwatchWorker  chan string
	failedWorker   chan string
	watchedWorkers map[string]chan struct{}
}

func init() {
	gob.Register(common.Ping{})
	gob.Register(common.Pong{})
}

func newFailureDetector() *failureDetector {
	return &failureDetector{
		shutdown:       make(chan struct{}),
		watchWorker:    make(chan workerInfo, 5),
		unwatchWorker:  make(chan string, 5),
		watchedWorkers: make(map[string]chan struct{}),
	}
}

func (fd *failureDetector) Shutdown() {
	log.Printf("Failure detector shutdown received. Cleaning up....")
	close(fd.shutdown)
}

func (fd *failureDetector) Start(failedWorker chan string) {
	fd.failedWorker = failedWorker
	for {
		select {
		case <-fd.shutdown:
			log.Printf("Failure detector shutdown")
			for workerId, done := range fd.watchedWorkers {
				log.Printf("Stopping failure detector for worker %v", workerId)
				close(done)
			}
			return

		case workerInfo := <-fd.watchWorker:
			if _, ok := fd.watchedWorkers[workerInfo.id]; ok {
				close(fd.watchedWorkers[workerInfo.id])
			}
			log.Printf("Watching worker %v", workerInfo.id)
			done := make(chan struct{})
			fd.watchedWorkers[workerInfo.id] = done
			go fd.monitorWorker(workerInfo, done)

		case workerId := <-fd.unwatchWorker:
			log.Printf("Un-watching worker %v", workerId)
			if _, ok := fd.watchedWorkers[workerId]; ok {
				log.Printf("Stopping failure detector for worker %v", workerId)
				close(fd.watchedWorkers[workerId])
				delete(fd.watchedWorkers, workerId)
			} else {
				log.Printf("Worker was not under observation %v", workerId)
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
			log.Printf("Failure detector for worker %v stopped", workerInfo.id)
			return
		case t := <-ticker.C:
			log.Printf("Sending heartbeat to %v at %v", workerInfo.id, t)
			if ok := heartbeat(workerInfo); !ok {
				ticker.Stop()
				fd.unwatchWorker <- workerInfo.id
				fd.failedWorker <- workerInfo.id
			}
		}
	}
}

func heartbeat(workerInfo workerInfo) bool {
	conn, err := net.Dial("tcp", workerInfo.address)
	if err != nil {
		log.Printf("Error connecting to worker: %v\n", err)
		return false
	}
	pingRequest := common.Ping{}

	var request interface{} = pingRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		log.Printf("Ping error to worker: %v\n", err)
		return false
	}
	conn.SetDeadline(time.Now().Add(1 * time.Second))
	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		log.Printf("Error decoding message: %v\n", err)
		return false
	}

	switch mt := response.(type) {
	case common.Pong:
		log.Printf("Pong received from %v (%v)\n", workerInfo.id, conn.RemoteAddr().String())
		return true
	default:
		log.Printf("Pong error, received message type %v from %v(%v): %v,\n", mt, workerInfo.id, conn.RemoteAddr().String(), response)
		return false
	}
}
