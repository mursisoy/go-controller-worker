package common

type Response struct {
	Success bool
	Message string
}

type SignupRequest struct {
	Address string
}

type SignupResponse struct {
	Response
}

type JobSubmitRequest struct {
	JobType string
	JobId   string
}

type JobSubmitResponse struct {
	Response
}

type JobDoneRequest struct {
	JobType string
	JobId   int
}

type JobDoneResponse struct {
	Response
}

type Ping struct{}
type Pong struct{}
