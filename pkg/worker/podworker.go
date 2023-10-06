package worker

import (
	"context"
	"fmt"
	"os"

	"metric-collector/pkg/api/kubelet"
	"metric-collector/pkg/decode"
	"metric-collector/pkg/storage"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type PodManager struct {
	MetricType string
}
type PodCollector struct {
	ClusterManager   *PodManager
	CPUCoreGauge     *prometheus.Desc
	CPUCoreCounter   *prometheus.Desc
	MemoryGauge      *prometheus.Desc
	MemoryCounter    *prometheus.Desc
	StorageGauge     *prometheus.Desc
	StorageCounter   *prometheus.Desc
	NetworkRXCounter *prometheus.Desc
	NetworkTXCounter *prometheus.Desc
	NetworkGauge     *prometheus.Desc
	ClientSet        *kubernetes.Clientset
	KubeletClient    *kubelet.KubeletClient
}

func NewPodManager(reg prometheus.Registerer, clientset *kubernetes.Clientset, kubeletClient *kubelet.KubeletClient) {
	ncm := &PodManager{
		MetricType: "PodMetric",
	}
	nc := PodCollector{
		ClusterManager: ncm,
		KubeletClient:  kubeletClient,
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
			"Network_RX_Counter",
			"Pod Network RX Utilization with Counter",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		NetworkTXCounter: prometheus.NewDesc(
			"Network_TX_Counter",
			"Pod Network TX Utilization with Counter",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		NetworkGauge: prometheus.NewDesc(
			"Network_Gauge",
			"Pod Network Utilization with Percent",
			[]string{"clustername", "podnamespace", "podname"}, nil,
		),
		ClientSet: clientset,
	}
	prometheus.WrapRegistererWith(prometheus.Labels{"metricType": ncm.MetricType}, reg).MustRegister(nc)
}

func (nc PodCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.CPUCoreCounter
	ch <- nc.CPUCoreGauge
	ch <- nc.MemoryCounter
	ch <- nc.MemoryGauge
	ch <- nc.NetworkRXCounter
	ch <- nc.NetworkTXCounter
	ch <- nc.NetworkGauge
	ch <- nc.StorageCounter
	ch <- nc.StorageGauge
}
func (nc PodCollector) Collect(ch chan<- prometheus.Metric) {
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

	collection, err := PodScrap(nc.KubeletClient, node)
	if err != nil {
		klog.Errorln(err)
	}
	clusterName := collection.ClusterName

	for _, podMetric := range collection.Metricsbatch.Pods {
		cpuUsage, _ := podMetric.CPUUsageNanoCores.AsInt64()
		cpuPercent := float64(cpuUsage) / float64(totalCPU)
		memoryUsage, _ := podMetric.MemoryUsageBytes.AsInt64()
		memoryPercent := float64(memoryUsage) / float64(totalMemory)
		storageUsage, _ := podMetric.FsUsedBytes.AsInt64()
		storagePercent := float64(storageUsage) / float64(totalStorage)
		networkrxUsage, _ := podMetric.NetworkRxBytes.AsInt64()
		networktxUsage, _ := podMetric.NetworkTxBytes.AsInt64()
		networkChange := podMetric.NetworkChange
		prevNetworkChange := podMetric.PrevNetworkChange
		networkPercent := float64(networkChange) / float64(prevNetworkChange)

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
			nc.NetworkGauge,
			prometheus.GaugeValue,
			float64(networkPercent*100),
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
	prevPods = collection.Metricsbatch.Pods
}

func PodScrap(kubelet_client *kubelet.KubeletClient, node *v1.Node) (*storage.Collection, error) {
	metrics, err := CollectPod(kubelet_client, node)
	if err != nil {
		klog.Errorf("unable to fully scrape metrics from node %s: %v", node.Name, err)
	}

	var errs []error
	res := &storage.Collection{
		Metricsbatch: metrics,
		ClusterName:  os.Getenv("CLUSTER_NAME"),
	}

	return res, utilerrors.NewAggregate(errs)
}

func CollectPod(kubelet_client *kubelet.KubeletClient, node *v1.Node) (*storage.MetricsBatch, error) {
	summary, err := kubelet_client.GetSummary()
	if err != nil {
		return nil, fmt.Errorf("unable to fetch metrics from Kubelet %s (%s): %v", node.Name, node.Status.Addresses[0].Address, err)
	}

	return decode.DecodePodBatch(summary, prevPods)
}
