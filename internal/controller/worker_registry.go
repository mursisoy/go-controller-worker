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
	id                string
	registeredWorkers map[workerId]workerInfo
	availableWorkers  map[workerId]struct{}
	registryMutex     sync.Mutex
}

func newWorkerRegistry(id string) *workerRegistry {
	return &workerRegistry{
		registeredWorkers: make(map[workerId]workerInfo),
		availableWorkers:  make(map[workerId]struct{}),
	}
}

func (wr *workerRegistry) registerWorker(wi workerInfo) bool {
	wr.registryMutex.Lock()
	defer wr.registryMutex.Unlock()
	if wr.isRegistered(wi.id) {
		return false
	}
	wr.registeredWorkers[wi.id] = wi
	wr.availableWorkers[wi.id] = struct{}{}
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
		return wr.registeredWorkers[worker], nil
	}
	return workerInfo{}, fmt.Errorf("no available workers at this moment")
}
