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
	// go workerReg.StartNodeServer(nodeName)
	workerReg.KETINodeRegistry.Gather()
	workerReg.KETIPodRegistry.Gather()
	collector.RunCollectorServer(workerReg.KETIPodRegistry, workerReg.KETINodeRegistry)
}
