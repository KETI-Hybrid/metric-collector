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
	ClusterManager   *NodeManager
	HostCPUUsage     *prometheus.Desc
	HostMemoryUsage  *prometheus.Desc
	HostNetworkUsage *prometheus.Desc
	HostFsUsage      *prometheus.Desc
	ClientSet        *kubernetes.Clientset
	KubeletClient    *kubelet.KubeletClient
}

func NewNodeManager(reg prometheus.Registerer, clientset *kubernetes.Clientset, kubeletClient *kubelet.KubeletClient) {
	ncm := &NodeManager{
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
		HostNetworkUsage: prometheus.NewDesc(
			"Host_Network_Usage",
			"Host Network Usage percent",
			[]string{"clustername", "nodename"}, nil,
		),
		HostFsUsage: prometheus.NewDesc(
			"Host_Storage_Usage",
			"Host Storage Usage Percent",
			[]string{"clustername", "nodename"}, nil,
		),
		ClientSet: clientset,
	}
	prometheus.WrapRegistererWith(prometheus.Labels{"metricType": ncm.MetricType}, reg).MustRegister(nc)
}

func (nc NodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.HostCPUUsage
	ch <- nc.HostFsUsage
	ch <- nc.HostMemoryUsage
	ch <- nc.HostNetworkUsage
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
	nodeNetworkChange := collection.Metricsbatch.Node.NetworkChange
	nodePrevNetworkChange := collection.Metricsbatch.Node.PrevNetworkChange
	nodeStorage, _ := collection.Metricsbatch.Node.FsUsedBytes.AsInt64()

	nodeCPUPercent := float64(nodeCores) / float64(totalCPU)
	nodeMemoryPercent := float64(nodeMemory) / float64(totalMemory)
	nodeNetworkPercent := float64(nodeNetworkChange) / float64(nodePrevNetworkChange)
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
		nc.HostNetworkUsage,
		prometheus.GaugeValue,
		float64(nodeNetworkPercent*100),
		clusterName, nodeName,
	)
	ch <- prometheus.MustNewConstMetric(
		nc.HostFsUsage,
		prometheus.GaugeValue,
		float64(nodeStoragePercent*100),
		clusterName, nodeName,
	)

	prevNode = collection.Metricsbatch.Node
}

func Scrap(kubelet_client *kubelet.KubeletClient, node *v1.Node) (*storage.Collection, error) {
	metrics, err := CollectNode(kubelet_client, node)
	if err != nil {
		err = fmt.Errorf("unable to fully scrape metrics from node %s: %v", node.Name, err)
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
