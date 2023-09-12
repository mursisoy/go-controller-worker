package controller

import (
	"encoding/gob"
	"log"
	"mursisoy/wordcount/internal/common"
	"net"
	"time"
)

type FailureDetector struct {
	shutdown       chan struct{}
	watchWorker    chan string
	unwatchWorker  chan string
	failedWorker   chan string
	watchedWorkers map[string]chan struct{}
}

func init() {
	gob.Register(common.Ping{})
	gob.Register(common.Pong{})
}

func newFailureDetector() *FailureDetector {
	return &FailureDetector{
		shutdown:       make(chan struct{}),
		watchWorker:    make(chan string),
		unwatchWorker:  make(chan string),
		watchedWorkers: make(map[string]chan struct{}),
	}
}

func (fd *FailureDetector) Shutdown() {
	log.Printf("Failure detector shutdown received. Cleaning up....")
	close(fd.shutdown)
}

func (fd *FailureDetector) Start(failedWorker chan string) {
	fd.failedWorker = failedWorker
	for {
		select {
		case <-fd.shutdown:
			log.Printf("Failure detector shutdown")
			for address, done := range fd.watchedWorkers {
				log.Printf("Stopping failure detector for worker %v", address)
				close(done)
			}
			return

		case workerAddress := <-fd.watchWorker:
			if _, ok := fd.watchedWorkers[workerAddress]; ok {
				close(fd.watchedWorkers[workerAddress])
			}
			log.Printf("Watching worker %v", workerAddress)
			done := make(chan struct{})
			fd.watchedWorkers[workerAddress] = done
			go fd.monitorWorker(workerAddress, done)

		case workerAddress := <-fd.unwatchWorker:
			log.Printf("Un-watching worker %v", workerAddress)
			if _, ok := fd.watchedWorkers[workerAddress]; ok {
				log.Printf("Stopping failure detector for worker %v", workerAddress)
				close(fd.watchedWorkers[workerAddress])
				delete(fd.watchedWorkers, workerAddress)
			} else {
				log.Printf("Worker was not under observation %v", workerAddress)
			}
		}

	}
}

func (fd *FailureDetector) monitorWorker(address string, done chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-done:
			ticker.Stop()
			log.Printf("Failure detector for worker %v stopped", address)
			return
		case t := <-ticker.C:
			log.Printf("Sending heartbeat to %v at %v", address, t)
			if ok := heartbeat(address); !ok {
				ticker.Stop()
				fd.failedWorker <- address
			}

		}
	}
}

func heartbeat(address string) bool {
	conn, err := net.Dial("tcp", address)
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
		log.Printf("Pong received from %v\n", address)
		return true
	default:
		log.Printf("Pong error, received message type %v from %v: %v,\n", mt, address, response)
		return false
	}
}
