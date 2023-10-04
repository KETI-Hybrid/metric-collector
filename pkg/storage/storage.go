package storage

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

type Collection struct {
	Metricsbatch *MetricsBatch
	ClusterName  string
}

// MetricsBatch is a single batch of pod, container, and node metrics from some source.
type MetricsBatch struct {
	IP   string
	Node NodeMetricsPoint
	Pods []PodMetricsPoint
}

// NodeMetricsPoint contains the metrics for some node at some point in time.
type NodeMetricsPoint struct {
	Name string
	MetricsPoint
}

// PodMetricsPoint contains the metrics for some pod's containers.
type PodMetricsPoint struct {
	Name      string
	Namespace string
	MetricsPoint
	Containers []ContainerMetricsPoint
}

// ContainerMetricsPoint contains the metrics for some container at some point in time.
type ContainerMetricsPoint struct {
	Name string
	MetricsPoint
}

// MetricsPoint represents the a set of specific metrics at some point in time.
type MetricsPoint struct {
	Timestamp time.Time

	// Cpu
	CPUUsageNanoCores resource.Quantity

	// Memory
	MemoryUsageBytes      resource.Quantity
	MemoryAvailableBytes  resource.Quantity
	MemoryWorkingSetBytes resource.Quantity

	// Network
	NetworkRxBytes resource.Quantity
	NetworkTxBytes resource.Quantity
	NetworkChange int64

	// Prev Network
	PrevNetworkRxBytes int64
	PrevNetworkTxBytes int64
	PrevNetworkChange  int64

	// Fs
	FsAvailableBytes resource.Quantity
	FsCapacityBytes  resource.Quantity
	FsUsedBytes      resource.Quantity
}

type PodGPU struct {
	PodName      string
	Pid          uint
	Index        int
	GPUmpscount  int
	Usegpumemory int
	ContainerID  string
	RunningCheck bool
	Isrunning    bool
	HealthCheck  bool
	CPU          int
	Memory       int
	Storage      int
	NetworkRX    int
	NetworkTX    int
}

type GPUMAP struct {
	GPUUUID   string
	PodMetric []PodGPU
}

type SlurmJob struct {
	JobName   string
	Index     int
	StartTime string
}

type NvlinkStatus struct {
	UUID          string
	BusID         string
	Lanes         map[string]int
	P2PUUID       []string
	P2PDeviceType []int //0 GPU, 1 IBMNPU, 2 SWITCH, 255 = UNKNOWN
	P2PBusID      []string
}
