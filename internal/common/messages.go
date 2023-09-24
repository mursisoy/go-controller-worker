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
	JobType string
	JobId   string
}

type JobSubmitResponse struct {
	Response
}

type JobDoneRequest struct {
	Request
	JobType string
	JobId   int
}

type JobDoneResponse struct {
	Response
}

type Ping struct {
	clock.ClockPayload
}
type Pong struct {
	clock.ClockPayload
}
