package main

import (
	"metric-collector/pkg/housekeeping"
	"metric-collector/pkg/image"
	"metric-collector/pkg/worker"
	"os"
)

func main() {
	run()
}

func run() {
	nodeName := os.Getenv("NODE_NAME")
	workerReg := worker.Initmetrics(nodeName)
	hk := housekeeping.NewHouseKeeper(workerReg.KetiClient, workerReg.NodeManager.ClientSet)
	im := image.NewImageManager(workerReg.KetiClient, workerReg.NodeManager.ClientSet, nodeName)
	go im.ImageWatcher()
	go hk.NodeKeeping()
	go hk.PodKeeping()
	go workerReg.PodManager.Collect()
	workerReg.NodeManager.Collect()

}
