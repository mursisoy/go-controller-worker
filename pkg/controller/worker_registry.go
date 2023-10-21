package controller

import (
	"fmt"
	"sync"
)

type workerId string
type workerAddress string

type workerInfo struct {
	id      workerId
	address workerAddress
}

type workerRegistry struct {
	registeredWorkers  map[workerId]workerInfo
	availableWorkers   map[workerId]struct{}
	registryMutex      sync.Mutex
	noAvailableWorkers chan struct{}
}

func newWorkerRegistry() *workerRegistry {
	return &workerRegistry{
		registeredWorkers:  make(map[workerId]workerInfo),
		availableWorkers:   make(map[workerId]struct{}),
		noAvailableWorkers: make(chan struct{}),
	}
}

func (wr *workerRegistry) registerWorker(wi workerInfo) bool {
	wr.registryMutex.Lock()
	defer wr.registryMutex.Unlock()
	if wr.isRegistered(wi.id) {
		return false
	}
	signalAvailableWorkers := (len(wr.availableWorkers) == 0)
	wr.registeredWorkers[wi.id] = wi
	wr.availableWorkers[wi.id] = struct{}{}
	if signalAvailableWorkers {
		close(wr.noAvailableWorkers)
	}
	return true
}

func (wr *workerRegistry) deregisterWorker(wid workerId) bool {
	wr.registryMutex.Lock()
	defer wr.registryMutex.Unlock()

	if !wr.isRegistered(wid) {
		return false
	}

	delete(wr.availableWorkers, wid)
	delete(wr.registeredWorkers, wid)
	if len(wr.availableWorkers) == 0 {
		wr.noAvailableWorkers = make(chan struct{})
	}
	return true
}

func (wr *workerRegistry) isAvailable(wid workerId) bool {
	if _, ok := wr.availableWorkers[wid]; ok {
		return true
	}
	return false
}

func (wr *workerRegistry) isRegistered(wid workerId) bool {
	if _, ok := wr.registeredWorkers[wid]; ok {
		return true
	}
	return false
}

func (wr *workerRegistry) getAvailableWorker() (workerInfo, error) {
	wr.registryMutex.Lock()
	defer wr.registryMutex.Unlock()
	for worker := range wr.availableWorkers {
		delete(wr.availableWorkers, worker)
		if len(wr.availableWorkers) == 0 {
			wr.noAvailableWorkers = make(chan struct{})
		}
		return wr.registeredWorkers[worker], nil
	}

	return workerInfo{}, fmt.Errorf("no available workers at this moment")
}

func (wr *workerRegistry) putAvailableWorker(wid workerId) error {
	wr.registryMutex.Lock()
	defer wr.registryMutex.Unlock()
	if !wr.isRegistered(wid) {
		return fmt.Errorf("worker %v is not registered", wid)
	}
	signalAvailableWorkers := (len(wr.availableWorkers) == 0)
	wr.availableWorkers[wid] = struct{}{}
	if signalAvailableWorkers {
		close(wr.noAvailableWorkers)
	}
	return nil
}
