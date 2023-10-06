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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var prevNode storage.NodeMetricsPoint
var prevPods []storage.PodMetricsPoint

type MetricWorker struct {
	KETINodeRegistry *prometheus.Registry
	KETIPodRegistry  *prometheus.Registry
	KubeletClient    *kubelet.KubeletClient
}

func Initmetrics(nodeName string) *MetricWorker {
	// Since we are dealing with custom Collector implementations, it might
	// be a good idea to try it out with a pedantic registry.
	fmt.Println("Initializing metrics...")

	worker := &MetricWorker{
		KETINodeRegistry: prometheus.NewRegistry(),
		KETIPodRegistry:  prometheus.NewRegistry(),
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
	NewNodeManager(worker.KETINodeRegistry, clientset, worker.KubeletClient)
	NewPodManager(worker.KETIPodRegistry, clientset, worker.KubeletClient)
	return worker
}

// func (mw *MetricWorker) StartNodeServer(nodeName string) {
// 	http.Handle("/nodemetric", promhttp.HandlerFor(mw.KETINodeRegistry, promhttp.HandlerOpts{
// 		EnableOpenMetrics: true,
// 	}))
// 	http.Handle("/podmetric", promhttp.HandlerFor(mw.KETIPodRegistry, promhttp.HandlerOpts{
// 		EnableOpenMetrics: true,
// 	}))
// 	log.Fatal(http.ListenAndServe(":9394", nil))
// }

type NodeManager struct {
	MetricType string
}
type NodeCollector struct {
	ClusterManager      *NodeManager
	HostCPUUsage        *prometheus.Desc
	HostCPUQuantity     *prometheus.Desc
	HostCPUPercent      *prometheus.Desc
	HostMemoryUsage     *prometheus.Desc
	HostMemoryQuantity  *prometheus.Desc
	HostMemoryPercent   *prometheus.Desc
	HostStoragyUsage    *prometheus.Desc
	HostStoragyQuantity *prometheus.Desc
	HostStoragyPercent  *prometheus.Desc
	HostNetworkRx       *prometheus.Desc
	HostNetworkTx       *prometheus.Desc
	ClientSet           *kubernetes.Clientset
	KubeletClient       *kubelet.KubeletClient
}

func NewNodeManager(reg prometheus.Registerer, clientset *kubernetes.Clientset, kubeletClient *kubelet.KubeletClient) {
	ncm := &NodeManager{
		MetricType: "NodeMetric",
	}
	nc := NodeCollector{
		ClusterManager: ncm,
		KubeletClient:  kubeletClient,
		HostCPUUsage: prometheus.NewDesc(
			"Host_CPU_Usage",
			"Node CPU Usage",
			[]string{"clustername", "nodename"}, nil,
		),
		HostCPUQuantity: prometheus.NewDesc(
			"Host_CPU_Quantity",
			"Node CPU Capacity",
			[]string{"clustername", "nodename"}, nil,
		),
		HostCPUPercent: prometheus.NewDesc(
			"Host_CPU_Percent",
			"Node CPU Usage Percentage",
			[]string{"clustername", "nodename"}, nil,
		),
		HostMemoryUsage: prometheus.NewDesc(
			"Host_Memory_Usage",
			"Host Memory Usage",
			[]string{"clustername", "nodename"}, nil,
		),
		HostMemoryQuantity: prometheus.NewDesc(
			"Host_Memory_Quantity",
			"Node Memory Capacity",
			[]string{"clustername", "nodename"}, nil,
		),
		HostMemoryPercent: prometheus.NewDesc(
			"Host_Memory_Percent",
			"Node Memory Usage Percentage",
			[]string{"clustername", "nodename"}, nil,
		),
		HostStoragyUsage: prometheus.NewDesc(
			"Host_Storage_Usage",
			"Node Storage Usage",
			[]string{"clustername", "nodename"}, nil,
		),
		HostStoragyQuantity: prometheus.NewDesc(
			"Host_Storage_Quantity",
			"Node Storagy Capacity",
			[]string{"clustername", "nodename"}, nil,
		),
		HostStoragyPercent: prometheus.NewDesc(
			"Host_Storage_Percent",
			"Node Storagy Usage Percentage",
			[]string{"clustername", "nodename"}, nil,
		),
		HostNetworkRx: prometheus.NewDesc(
			"Host_Network_RX_Bytes",
			"Node Network Reading Bytes",
			[]string{"clustername", "nodename"}, nil,
		),
		HostNetworkTx: prometheus.NewDesc(
			"Host_Network_TX_Bytes",
			"Node Network Transmission Bytes",
			[]string{"clustername", "nodename"}, nil,
		),
		ClientSet: clientset,
	}
	prometheus.WrapRegistererWith(prometheus.Labels{"metricType": ncm.MetricType}, reg).MustRegister(nc)
}

func (nc NodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.HostCPUUsage
	ch <- nc.HostCPUQuantity
	ch <- nc.HostCPUPercent
	ch <- nc.HostMemoryUsage
	ch <- nc.HostMemoryQuantity
	ch <- nc.HostMemoryPercent
	ch <- nc.HostStoragyUsage
	ch <- nc.HostStoragyQuantity
	ch <- nc.HostStoragyPercent
	ch <- nc.HostNetworkRx
	ch <- nc.HostNetworkTx
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
	prevRx := collection.Metricsbatch.Node.PrevNetworkRxBytes
	prevTx := collection.Metricsbatch.Node.PrevNetworkTxBytes
	nodeNetworkRx, _ := collection.Metricsbatch.Node.NetworkRxBytes.AsInt64()
	nodeNetworkRx = nodeNetworkRx - prevRx
	nodeNetworkTx, _ := collection.Metricsbatch.Node.NetworkTxBytes.AsInt64()
	nodeNetworkTx = nodeNetworkTx - prevTx
	nodeStorage, _ := collection.Metricsbatch.Node.FsUsedBytes.AsInt64()

	nodeCPUPercent := float64(nodeCores) / float64(totalCPU)
	nodeMemoryPercent := float64(nodeMemory) / float64(totalMemory)
	nodeStoragePercent := float64(nodeStorage) / float64(totalStorage)
	ch <- prometheus.MustNewConstMetric(
		nc.HostCPUUsage,
		prometheus.GaugeValue,
		float64(nodeCores),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostCPUQuantity,
		prometheus.GaugeValue,
		float64(totalCPU),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostCPUPercent,
		prometheus.GaugeValue,
		float64(nodeCPUPercent),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostMemoryUsage,
		prometheus.GaugeValue,
		float64(nodeMemory),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostMemoryQuantity,
		prometheus.GaugeValue,
		float64(totalMemory),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostMemoryPercent,
		prometheus.GaugeValue,
		float64(nodeMemoryPercent),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostStoragyUsage,
		prometheus.GaugeValue,
		float64(nodeStorage),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostStoragyQuantity,
		prometheus.GaugeValue,
		float64(totalStorage),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostStoragyPercent,
		prometheus.GaugeValue,
		float64(nodeStoragePercent),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostNetworkRx,
		prometheus.GaugeValue,
		float64(nodeNetworkRx),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostNetworkTx,
		prometheus.GaugeValue,
		float64(nodeNetworkTx),
		clusterName, nodeName,
	)

	prevNode = collection.Metricsbatch.Node
}

func Scrap(kubelet_client *kubelet.KubeletClient, node *v1.Node) (*storage.Collection, error) {
	metrics, err := CollectNode(kubelet_client, node)
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

func CollectNode(kubelet_client *kubelet.KubeletClient, node *v1.Node) (*storage.MetricsBatch, error) {

	summary, err := kubelet_client.GetSummary()
	if err != nil {
		return nil, fmt.Errorf("unable to fetch metrics from Kubelet %s (%s): %v", node.Name, node.Status.Addresses[0].Address, err)
	}

	return decode.DecodeNodeBatch(summary, prevNode)
}
