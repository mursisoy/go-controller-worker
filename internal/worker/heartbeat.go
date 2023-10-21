package worker

import (
	"encoding/gob"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/common"
	"net"
)

func init() {
	gob.Register(common.Ping{})
	gob.Register(common.Pong{})
}

func (w *Worker) handleHeartbeat(pingRequest common.Ping, conn net.Conn) {
	w.clog.LogMergeInfof(pingRequest.Clock, "new ping request from %s (%v)", pingRequest.Pid, conn.RemoteAddr())

	cc := w.clog.LogInfof("send pong to %s (%v)", pingRequest.Pid, conn.RemoteAddr())
	var pongResponse = common.Pong{
		ClockPayload: clock.ClockPayload{Clock: cc, Pid: w.pid},
	}
	encoder := gob.NewEncoder(conn)
	var response interface{} = pongResponse
	if err := encoder.Encode(&response); err != nil {
		w.clog.LogErrorf("pong error to server: %v", err)
	}
}
