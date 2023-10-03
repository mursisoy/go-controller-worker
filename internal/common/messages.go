package common

import "mursisoy/wordcount/internal/clock"

type Request struct {
	clock.ClockPayload
}

func RequestWithClock(pid string, cc clock.ClockMap) Request {
	return Request{
		ClockPayload: clock.ClockPayload{
			Pid:   pid,
			Clock: cc,
		},
	}
}

func ResponseWithClock(pid string, cc clock.ClockMap, success bool) Response {
	return Response{
		ClockPayload: clock.ClockPayload{
			Pid:   pid,
			Clock: cc,
		},
		Success: success,
	}
}

type Response struct {
	clock.ClockPayload
	Success bool
	Message string
}

type SignupRequest struct {
	Request
	Id      string
	Address string
}

type SignupResponse struct {
	Response
}

type JobSubmitRequest struct {
	Request
	Job Job
}
type JobSubmitResponse struct {
	Response
	Job Job
}

type TaskSubmitRequest struct {
	Request
	Task Task
}
type TaskSubmitResponse struct {
	Response
}

type TaskDoneRequest struct {
	Request
	Task Task
}
type TaskDoneResponse struct {
	Response
}

type JobStatusRequest struct {
	Request
}

type JobStatusResponse struct {
	Response
	Job
}

type JobDoneRequest struct {
	Request
	JobType string
	JobId   int
}

type Ping struct {
	clock.ClockPayload
}
type Pong struct {
	clock.ClockPayload
}
