package worker

import (
	"testing"
	"time"

	"github.com/mursisoy/go-controller-worker/pkg/clock"
	"github.com/mursisoy/go-controller-worker/pkg/controller"
)

func TestSignup(t *testing.T) {
	controllerConfig := controller.ControllerConfig{
		ListenAddress: ":0",
		LogPriority:   clock.DEBUG,
		FailureDetectorConfig: controller.FailureDetectorConfig{
			HeartBeatTicker: 2 * time.Second,
		},
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
