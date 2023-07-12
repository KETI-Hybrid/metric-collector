package worker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"metric-collector/pkg/api/kubelet"
	"metric-collector/pkg/decode"
	"metric-collector/pkg/storage"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type MetricWorker struct {
	KETIRegistry  *prometheus.Registry
	KubeletClient *kubelet.KubeletClient
}

func Initmetrics(nodeName string) *MetricWorker {
	// Since we are dealing with custom Collector implementations, it might
	// be a good idea to try it out with a pedantic registry.
	fmt.Println("Initializing metrics...")

	worker := &MetricWorker{
		KETIRegistry: prometheus.NewRegistry(),
	}

	//reg := prometheus.NewPedanticRegistry()
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Println(err.Error())
		config, err = clientcmd.BuildConfigFromFlags("", "/root/workspace/metric-collector/config/config")
		if err != nil {
			fmt.Println(err.Error())
			return nil
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Error: %v\n", err)
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			worker.KubeletClient = kubelet.NewKubeletClient(addr.Address, config.BearerToken)
			break
		}
	}
	NewClusterManager(worker.KETIRegistry, clientset, worker.KubeletClient)
	return worker
}
func (mw *MetricWorker) StartRestServer(nodeName string) {
	http.Handle("/"+nodeName+"/metric", promhttp.HandlerFor(mw.KETIRegistry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	log.Fatal(http.ListenAndServe(":9394", nil))
}

type ClusterManager struct {
	MetricType string
}
type NodeCollector struct {
	ClusterManager     *ClusterManager
	HostCPUUsage       *prometheus.Desc
	HostMemoryUsage    *prometheus.Desc
	HostNetworkRxBytes *prometheus.Desc
	HostNetworkTxBytes *prometheus.Desc
	HostFsUsage        *prometheus.Desc
	CPUCoreGauge       *prometheus.Desc
	CPUCoreCounter     *prometheus.Desc
	MemoryGauge        *prometheus.Desc
	MemoryCounter      *prometheus.Desc
	StorageGauge       *prometheus.Desc
	StorageCounter     *prometheus.Desc
	NetworkRXCounter   *prometheus.Desc
	NetworkTXCounter   *prometheus.Desc
	ClientSet          *kubernetes.Clientset
	KubeletClient      *kubelet.KubeletClient
}

func NewClusterManager(reg prometheus.Registerer, clientset *kubernetes.Clientset, kubeletClient *kubelet.KubeletClient) {
	ncm := &ClusterManager{
		MetricType: "NodeMetric",
	}
	nc := NodeCollector{
		ClusterManager: ncm,
		KubeletClient:  kubeletClient,
		HostCPUUsage: prometheus.NewDesc(
			"Host_CPU_Core_Usage",
			"Node CPU Usage Percent",
			[]string{"clustername", "nodename"}, nil,
		),
		HostMemoryUsage: prometheus.NewDesc(
			"Host_Memory_Usage",
			"Host Memory Usage percent",
			[]string{"clustername", "nodename"}, nil,
		),
		HostNetworkRxBytes: prometheus.NewDesc(
			"Host_Network_Rx_Byte",
			"Host Network Recieve Byte",
			[]string{"clustername", "nodename"}, nil,
		),
		HostNetworkTxBytes: prometheus.NewDesc(
			"Host_Network_Tx_Byte",
			"Host Network Transport Byte",
			[]string{"clustername", "nodename"}, nil,
		),
		HostFsUsage: prometheus.NewDesc(
			"Host_Storage_Usage",
			"Host Storage Usage Percent",
			[]string{"clustername", "nodename"}, nil,
		),
		CPUCoreGauge: prometheus.NewDesc(
			"CPU_Core_Gauge",
			"Pod CPU Utilization with Percent",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		CPUCoreCounter: prometheus.NewDesc(
			"CPU_Core_Counter",
			"Pod CPU Utilization with Counter",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		MemoryGauge: prometheus.NewDesc(
			"Memory_Gauge",
			"Pod Memory Utilization with Percent",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		MemoryCounter: prometheus.NewDesc(
			"Memory_Counter",
			"Pod Memory Utilization with Counter",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		StorageGauge: prometheus.NewDesc(
			"Storage_Gauge",
			"Pod Storage Utilization with Percent",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		StorageCounter: prometheus.NewDesc(
			"Storage_Counter",
			"Pod Storage Utilization with Counter",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		NetworkRXCounter: prometheus.NewDesc(
			"Network_Gauge",
			"Pod Network RX Utilization with Counter",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		NetworkTXCounter: prometheus.NewDesc(
			"Network_Counter",
			"Pod Network TX Utilization with Counter",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		ClientSet: clientset,
	}
	prometheus.WrapRegistererWith(prometheus.Labels{"metricType": ncm.MetricType}, reg).MustRegister(nc)
}

func (nc NodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.HostCPUUsage
	ch <- nc.HostFsUsage
	ch <- nc.HostMemoryUsage
	ch <- nc.HostNetworkRxBytes
	ch <- nc.HostNetworkTxBytes
	ch <- nc.CPUCoreCounter
	ch <- nc.CPUCoreGauge
	ch <- nc.MemoryCounter
	ch <- nc.MemoryGauge
	ch <- nc.NetworkRXCounter
	ch <- nc.NetworkTXCounter
	ch <- nc.StorageCounter
	ch <- nc.StorageGauge
}
func (nc NodeCollector) Collect(ch chan<- prometheus.Metric) {
	nodeName := os.Getenv("NODE_NAME")
	node, err := nc.ClientSet.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorln(err)
	}

	totalCPUQuantity := node.Status.Allocatable["cpu"]
	totalCPU, _ := totalCPUQuantity.AsInt64()
	totalMemoryQuantity := node.Status.Allocatable["memory"]
	totalMemory, _ := totalMemoryQuantity.AsInt64()
	totalStorageQuantity := node.Status.Allocatable["ephemeral-storage"]
	totalStorage, _ := totalStorageQuantity.AsInt64()

	collection, err := Scrap(nc.KubeletClient, node)
	if err != nil {
		klog.Errorln(err)
	}
	clusterName := collection.ClusterName
	nodeCores, _ := collection.Metricsbatch.Node.CPUUsageNanoCores.AsInt64()
	nodeMemory, _ := collection.Metricsbatch.Node.MemoryUsageBytes.AsInt64()
	nodeNetworkTx, _ := collection.Metricsbatch.Node.NetworkTxBytes.AsInt64()
	nodeNetworkRx, _ := collection.Metricsbatch.Node.NetworkRxBytes.AsInt64()
	nodeStorage, _ := collection.Metricsbatch.Node.FsUsedBytes.AsInt64()

	nodeCPUPercent := float64(nodeCores) / float64(totalCPU)
	nodeMemoryPercent := float64(nodeMemory) / float64(totalMemory)
	nodeStoragePercent := float64(nodeStorage) / float64(totalStorage)
	ch <- prometheus.MustNewConstMetric(
		nc.HostCPUUsage,
		prometheus.GaugeValue,
		float64(nodeCPUPercent*100),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostMemoryUsage,
		prometheus.GaugeValue,
		float64(nodeMemoryPercent*100),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostNetworkRxBytes,
		prometheus.GaugeValue,
		float64(nodeNetworkTx),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostNetworkTxBytes,
		prometheus.GaugeValue,
		float64(nodeNetworkRx),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostFsUsage,
		prometheus.GaugeValue,
		float64(nodeStoragePercent*100),
		clusterName, nodeName,
	)

	for _, podMetric := range collection.Metricsbatch.Pods {
		cpuUsage, _ := podMetric.CPUUsageNanoCores.AsInt64()
		cpuPercent := float64(cpuUsage) / float64(totalCPU)
		memoryUsage, _ := podMetric.MemoryUsageBytes.AsInt64()
		memoryPercent := float64(memoryUsage) / float64(totalMemory)
		storageUsage, _ := podMetric.FsUsedBytes.AsInt64()
		storagePercent := float64(storageUsage) / float64(totalStorage)

		networkrxUsage, _ := podMetric.NetworkRxBytes.AsInt64()
		networktxUsage, _ := podMetric.NetworkTxBytes.AsInt64()

		ch <- prometheus.MustNewConstMetric(
			nc.CPUCoreGauge,
			prometheus.GaugeValue,
			float64(cpuPercent*100),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			nc.CPUCoreCounter,
			prometheus.GaugeValue,
			float64(cpuUsage),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			nc.MemoryCounter,
			prometheus.GaugeValue,
			float64(memoryUsage),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			nc.MemoryGauge,
			prometheus.GaugeValue,
			float64(memoryPercent*100),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			nc.NetworkRXCounter,
			prometheus.GaugeValue,
			float64(networkrxUsage),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			nc.NetworkTXCounter,
			prometheus.GaugeValue,
			float64(networktxUsage),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			nc.StorageCounter,
			prometheus.GaugeValue,
			float64(storageUsage),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			nc.StorageGauge,
			prometheus.GaugeValue,
			float64(storagePercent*100),
			clusterName, podMetric.Namespace, podMetric.Name,
		)
	}

}

func Scrap(kubelet_client *kubelet.KubeletClient, node *v1.Node) (*storage.Collection, error) {
	//fmt.Println("Func Scrap Called")

	//startTime := clock.MyClock.Now()

	metrics, err := CollectNode(kubelet_client, node)
	if err != nil {
		err = fmt.Errorf("unable to fully scrape metrics from node %s: %v", node.Name, err)
	}

	var errs []error
	res := &storage.Collection{
		Metricsbatch: metrics,
		ClusterName:  os.Getenv("CLUSTER_NAME"),
	}

	//fmt.Println("ScrapeMetrics: time: ", clock.MyClock.Since(startTime), "nodes: ", nodeNum, "pods: ", podNum)
	return res, utilerrors.NewAggregate(errs)
}

func CollectNode(kubelet_client *kubelet.KubeletClient, node *v1.Node) (*storage.MetricsBatch, error) {

	summary, err := kubelet_client.GetSummary()
	//fmt.Println("summary : ", summary)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch metrics from Kubelet %s (%s): %v", node.Name, node.Status.Addresses[0].Address, err)
	}

	return decode.DecodeBatch(summary)
}

func GetGPUProcess(device nvml.Device) map[string]uint64 {
	nvmlProc, _ := device.GetComputeRunningProcesses()
	nvmlmpsProc, _ := device.GetMPSComputeRunningProcesses()
	nvmlProc = append(nvmlProc, nvmlmpsProc...)

	procInfo := make(map[string]uint64)

	for _, proc := range nvmlProc {
		strPid := fmt.Sprint(proc.Pid)
		procInfo[strPid] = proc.UsedGpuMemory
	}
	return procInfo
}
func getMemoryBandwidth(device nvml.Device) float64 {
	busWidth, _ := device.GetMemoryBusWidth()

	memClock, _ := device.GetMaxClockInfo(nvml.CLOCK_MEM)

	bandwidth := float64(busWidth) * float64(memClock) * 2 / 8 / 1e6 // GB/s
	return bandwidth
}
