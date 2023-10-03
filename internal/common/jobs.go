package common

import "time"

// LogPriority controls the minimum priority of logging events which
// will be logged.
type JobId uint32

type JobStatus int

const (
	QUEUE JobStatus = iota
	RUN
	DONE
)

var JobStatusDescription = [...]string{
	QUEUE: "QUEUE",
	RUN:   "RUN",
	DONE:  "DONE",
}

type Job struct {
	Id       JobId
	Duration time.Duration
}

type Task struct {
	JobId     JobId
	Duration  time.Duration
	WillFail  time.Duration
	WillCrash time.Duration
	Result    time.Time
}
