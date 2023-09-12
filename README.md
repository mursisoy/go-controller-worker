# Word Count in Golang

Given a master node, submit works to it.
The master node has a register with all the counter nodes.

## Failure detector
The controller has associated a failure detector. When a worker signs-up within a controller, the controller request the failure detector (through channels) to watch the worker (through TCP).

The failure detector initiates goroutines for each worker requested and saves a map with the address and a close channel to the function. The `monitorWorker` uses a ticker to send a heartbeat (Ping- Pong) to the worker. If this communication fails at any point, the failure detector, stops the ticker, unwatch the worker and notify the controller (through channel).
