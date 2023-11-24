package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"metric-collector/pkg/api/kubelet"
	"metric-collector/pkg/crd"
	"metric-collector/pkg/decode"
	"metric-collector/pkg/storage"

	levelv1 "github.com/KETI-Hybrid/keti-controller/apis/level/v1"
	keticlient "github.com/KETI-Hybrid/keti-controller/client"
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

const (
	Nano = 10e-9
	Giga = 1024 * 1024 * 1024
)

type MetricWorker struct {
	KETINodeRegistry *prometheus.Registry
	KETIPodRegistry  *prometheus.Registry
	KubeletClient    *kubelet.KubeletClient
	KetiClient       *keticlient.ClientSet
	NodeManager      *NodeCollector
	PodManager       *PodCollector
}

func Initmetrics(nodeName string) *MetricWorker {
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
	worker.KetiClient, err = crd.NewClient()
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
	worker.NodeManager = NewNodeManager(worker.KETINodeRegistry, clientset, worker.KubeletClient, worker.KetiClient)
	worker.PodManager = NewPodManager(worker.KETIPodRegistry, clientset, worker.KubeletClient, worker.KetiClient)
	return worker
}

type NodeCollector struct {
	ClientSet     *kubernetes.Clientset
	KubeletClient *kubelet.KubeletClient
	KetiClient    *keticlient.ClientSet
}

func NewNodeManager(reg prometheus.Registerer, clientset *kubernetes.Clientset, kubeletClient *kubelet.KubeletClient, ketiClient *keticlient.ClientSet) *NodeCollector {
	return &NodeCollector{
		KubeletClient: kubeletClient,
		ClientSet:     clientset,
		KetiClient:    ketiClient,
	}
}

func (nc NodeCollector) Collect() {
	nodeName := os.Getenv("NODE_NAME")
	for {
		node, err := nc.ClientSet.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Errorln(err)
		}
		currentNodeMetric := &levelv1.NodeMetric{
			TypeMeta: metav1.TypeMeta{
				Kind:       "NodeMetric",
				APIVersion: "level.hybrid.keti/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: node.Name,
			},
		}
		createFlag := false
		getCurrentNodeMetric, err := nc.KetiClient.LevelV1().NodeMetrics().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Errorln(err)
		}
		if len(getCurrentNodeMetric.Name) == 0 {
			createFlag = true
		} else {
			createFlag = false
			currentNodeMetric = getCurrentNodeMetric
		}
		totalCPUQuantity := node.Status.Capacity["cpu"]
		totalCPU, _ := totalCPUQuantity.AsInt64()
		totalMemoryQuantity := node.Status.Capacity["memory"]
		totalMemory, _ := totalMemoryQuantity.AsInt64()
		totalStorageQuantity := node.Status.Capacity["ephemeral-storage"]
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

		nodeCPUPercent := (float64(nodeCores) * Nano) / float64(totalCPU)
		nodeMemoryPercent := float64(nodeMemory) / float64(totalMemory)
		nodeStoragePercent := float64(nodeStorage) / float64(totalStorage)

		fmt.Println("Node core usage : ", (float64(nodeCores)*Nano)/40)
		fmt.Println("Node core capacity : ", totalCPU)

		currentNodeMetric.Spec.HostCPUUsage.Value = fmt.Sprintf("%.2f", ((float64(nodeCores) * 10e-9) / 40))
		currentNodeMetric.Spec.HostCPUUsage.ClusterName = clusterName
		currentNodeMetric.Spec.HostCPUUsage.NodeName = nodeName

		prevNode = collection.Metricsbatch.Node

		if currentNodeMetric.Labels == nil {
			currentNodeMetric.Labels = make(map[string]string)
		}

		currentNodeMetric.Labels["lastCollectTime"] = time.Now().Format("2006-01-02_15_04_05.0000")

		if createFlag {
			_, err = nc.KetiClient.LevelV1().NodeMetrics().Create(currentNodeMetric)
			if err != nil {
				klog.Errorln(err)
			}
		} else {
			_, err = nc.KetiClient.LevelV1().NodeMetrics().Update(currentNodeMetric)
			if err != nil {
				klog.Errorln(err)
			}
		}
		time.Sleep(time.Second * 5)
	}
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
