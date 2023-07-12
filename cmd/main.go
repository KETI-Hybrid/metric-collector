package main

import (
	"metric-collector/pkg/collector"
	"metric-collector/pkg/worker"
	"os"
)

func main() {
	run()
}

func run() {
	//isGPU := os.Getenv("IS_GPUNODE")
	nodeName := os.Getenv("NODE_NAME")
	workerReg := worker.Initmetrics(nodeName)
	go workerReg.StartRestServer(nodeName)
	collector.RunCollectorServer(workerReg.KETIRegistry)
}
