package common

type Signup struct {
	Address string
}

type MonitorWorker struct {
	Address string
}

type JobSubmit struct {
	JobType string
	JobId   string
}

type JobDone struct {
	JobType string
	JobId   int
}

type WorkerFailure struct {
	Address string
}

type Ping struct{}
type Pong struct{}
