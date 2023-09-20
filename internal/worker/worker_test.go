package worker

import (
	"mursisoy/wordcount/internal/controller"
	"testing"
)

func TestSignup(t *testing.T) {
	controllerConfig := controller.ControllerConfig{
		ListenAddress: ":0",
	}
	controller := controller.NewController("controller", controllerConfig)
	controllerAddr, err := controller.Start()
	if err != nil {
		t.Errorf("Signup failed: %v", err)
	}
	defer controller.Shutdown()

	workerConfig := WorkerConfig{
		ControllerAddress: controllerAddr.String(),
		ListenAddress:     ":0",
	}

	worker := NewWorker("testWorker", workerConfig)

	if err := worker.signup(); err != nil {
		t.Errorf("Signup failed: %v", err)
	}

}
