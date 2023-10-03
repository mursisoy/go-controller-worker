package main

import (
	"encoding/gob"
	"flag"
	"log"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/common"
	"net"
	"time"
)

// import (
//
//	"flag"
//	"log"
//	"mursisoy/wordcount/internal/clock"
//	"mursisoy/wordcount/internal/controller"
//	"os/signal"
//	"sync"
//	"syscall"
//	"time"
//
// )
func init() {
	gob.Register(common.JobSubmitResponse{})
	gob.Register(common.JobSubmitRequest{})
}

func main() {

	// args := flag.Args()
	// if len(args) == 0 {
	// 	flag.PrintDefaults()
	// 	log.Fatal("Please specify a subcommand.")
	// }

	// submitCmd := flag.NewFlagSet("submit", flag.ExitOnError)
	// duration := submitCmd.String("duration", "30s", "Job duration")

	// statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	// jobId := statusCmd.Int("job-id", -1, "The job id to ask for")

	var durationString, controllerAddress string

	flag.StringVar(&durationString, "duration", "30s", "The job duration")
	var duration time.Duration
	var err error
	if duration, err = time.ParseDuration(durationString); err != nil {
		flag.PrintDefaults()
		log.Fatalf("Unable to parse duration: %v", err)
	}

	flag.StringVar(&controllerAddress, "controller", "", "The controller address")

	// Enable command-line parsing
	flag.Parse()

	conn, err := net.Dial("tcp", controllerAddress)
	if err != nil {
		log.Fatalf("Error connecting to controller: %v", err)
	}
	jobSubmitRequest := common.JobSubmitRequest{
		Request: common.Request{
			ClockPayload: clock.ClockPayload{
				Pid: "js",
			},
		},
		Job: common.Job{
			Duration: duration,
		},
	}

	var request interface{} = jobSubmitRequest
	encoder := gob.NewEncoder(conn)
	if err = encoder.Encode(&request); err != nil {
		log.Fatalf("Error sending job to worker: %v", err)
	}

	conn.SetDeadline(time.Now().Add(1 * time.Second))
	var response interface{}
	decoder := gob.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		log.Fatalf("Error decoding message: %v", err)
	}

	switch mt := response.(type) {
	case common.JobSubmitResponse:
		if mt.Response.Success {
			log.Printf("Received success job response with job id %v", mt.Job.Id)
		} else {
			log.Fatalf("Received unsucessful response from controller")
		}
	default:
		log.Fatalf("Received  unknown response from controller")
	}

}
