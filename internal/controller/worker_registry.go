package controller

import (
	"errors"
	"fmt"
	"sync"
)

type workerInfo struct {
	id      string
	address string
}

type workerRegistry struct {
	registeredWorkers map[string]workerInfo
	availableWorkers  map[string]struct{}
	registryMutex     sync.Mutex
}

func newWorkerRegistry() *workerRegistry {
	return &workerRegistry{
		registeredWorkers: make(map[string]workerInfo),
		availableWorkers:  make(map[string]struct{}),
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

func (wr *workerRegistry) deregisterWorker(workerId string) bool {
	wr.registryMutex.Lock()
	defer wr.registryMutex.Unlock()

	if !wr.isRegistered(workerId) {
		return false
	}

	delete(wr.availableWorkers, workerId)
	delete(wr.registeredWorkers, workerId)
	return true
}

func (wr *workerRegistry) isAvailable(workerId string) bool {
	if _, ok := wr.availableWorkers[workerId]; ok {
		return true
	}
	return false
}

func (wr *workerRegistry) isRegistered(workerId string) bool {
	if _, ok := wr.registeredWorkers[workerId]; ok {
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
	return workerInfo{}, errors.New(fmt.Sprintf("No available workers at this moment"))
}
