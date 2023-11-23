package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"metric-collector/pkg/api/kubelet"
	"metric-collector/pkg/decode"
	"metric-collector/pkg/storage"

	levelv1 "github.com/KETI-Hybrid/keti-controller/apis/level/v1"
	keticlient "github.com/KETI-Hybrid/keti-controller/client"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type PodCollector struct {
	ClientSet     *kubernetes.Clientset
	KubeletClient *kubelet.KubeletClient
	KetiClient    *keticlient.ClientSet
}

func NewPodManager(reg prometheus.Registerer, clientset *kubernetes.Clientset, kubeletClient *kubelet.KubeletClient, ketiClient *keticlient.ClientSet) *PodCollector {
	return &PodCollector{
		KubeletClient: kubeletClient,
		ClientSet:     clientset,
		KetiClient:    ketiClient,
	}
}

func (nc PodCollector) Collect() {
	for {
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
			if podMetric.Namespace == "cdi" || podMetric.Namespace == "keti-controller-system" || podMetric.Namespace == "keti-system" || podMetric.Namespace == "kube-flannel" || podMetric.Namespace == "kube-node-lease" || podMetric.Namespace == "kube-public" || podMetric.Namespace == "kube-system" || podMetric.Namespace == "kubevirt" {
				continue
			}

			cpuUsage, _ := podMetric.CPUUsageNanoCores.AsInt64()
			cpuPercent := (float64(cpuUsage) * Nano) / float64(totalCPU)
			memoryUsage, _ := podMetric.MemoryUsageBytes.AsInt64()
			memoryPercent := float64(memoryUsage) / float64(totalMemory)
			storageUsage, _ := podMetric.FsUsedBytes.AsInt64()
			storagePercent := float64(storageUsage) / float64(totalStorage)
			prevRx := podMetric.PrevNetworkRxBytes
			prevTx := podMetric.PrevNetworkTxBytes
			networkrxUsage, _ := podMetric.NetworkRxBytes.AsInt64()
			networkrxUsage = networkrxUsage - prevRx
			networktxUsage, _ := podMetric.NetworkTxBytes.AsInt64()
			networktxUsage = networktxUsage - prevTx

			currentPodMetric := &levelv1.PodMetric{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PodMetric",
					APIVersion: "level.hybrid.keti/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   podMetric.Name,
					Labels: make(map[string]string),
				},
			}
			
			createFlag := false
			getCurrentPodMetric, err := nc.KetiClient.LevelV1().PodMetrics().Get(podMetric.Name, metav1.GetOptions{})
			
			if err != nil {
				klog.Errorln(err)
			}
			if len(getCurrentPodMetric.Name) == 0 {
				createFlag = true
			} else {
				createFlag = false
				currentPodMetric = getCurrentPodMetric
			}

			currentPodMetric.Spec.CPUCoreGauge.Value = fmt.Sprintf("%.2f", float64(cpuPercent*100))
			currentPodMetric.Spec.CPUCoreGauge.ClusterName = clusterName
			currentPodMetric.Spec.CPUCoreGauge.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.CPUCoreGauge.PodName = podMetric.Name

			currentPodMetric.Spec.CPUCoreCounter.Value = fmt.Sprintf("%.2f", (float64(cpuUsage) * Nano))
			currentPodMetric.Spec.CPUCoreCounter.ClusterName = clusterName
			currentPodMetric.Spec.CPUCoreCounter.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.CPUCoreCounter.PodName = podMetric.Name

			currentPodMetric.Spec.MemoryCounter.Value = fmt.Sprintf("%.2f", (float64(memoryUsage) / Giga))
			currentPodMetric.Spec.MemoryCounter.ClusterName = clusterName
			currentPodMetric.Spec.MemoryCounter.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.MemoryCounter.PodName = podMetric.Name

			currentPodMetric.Spec.MemoryGauge.Value = fmt.Sprintf("%.2f", float64(memoryPercent*100))
			currentPodMetric.Spec.MemoryGauge.ClusterName = clusterName
			currentPodMetric.Spec.MemoryGauge.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.MemoryGauge.PodName = podMetric.Name

			currentPodMetric.Spec.NetworkRXCounter.Value = fmt.Sprintf("%.2f", float64(networkrxUsage))
			currentPodMetric.Spec.NetworkRXCounter.ClusterName = clusterName
			currentPodMetric.Spec.NetworkRXCounter.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.NetworkRXCounter.PodName = podMetric.Name

			currentPodMetric.Spec.NetworkTXCounter.Value = fmt.Sprintf("%.2f", float64(networktxUsage))
			currentPodMetric.Spec.NetworkTXCounter.ClusterName = clusterName
			currentPodMetric.Spec.NetworkTXCounter.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.NetworkTXCounter.PodName = podMetric.Name

			currentPodMetric.Spec.NetworkGauge.Value = fmt.Sprintf("%.2f", float64(totalStorage))
			currentPodMetric.Spec.NetworkGauge.ClusterName = clusterName
			currentPodMetric.Spec.NetworkGauge.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.NetworkGauge.PodName = podMetric.Name

			currentPodMetric.Spec.StorageCounter.Value = fmt.Sprintf("%.2f", (float64(storageUsage) / Giga))
			currentPodMetric.Spec.StorageCounter.ClusterName = clusterName
			currentPodMetric.Spec.StorageCounter.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.StorageCounter.PodName = podMetric.Name

			currentPodMetric.Spec.StorageGauge.Value = fmt.Sprintf("%.2f", float64(storagePercent*100))
			currentPodMetric.Spec.StorageGauge.ClusterName = clusterName
			currentPodMetric.Spec.StorageGauge.PodNamespace = podMetric.Namespace
			currentPodMetric.Spec.StorageGauge.PodName = podMetric.Name

			if currentPodMetric.Labels == nil {
				currentPodMetric.Labels = make(map[string]string)
			}

			currentPodMetric.Labels["lastCollectTime"] = time.Now().Format("2006-01-02_15_04_05.0000")

			if createFlag {
				_, err = nc.KetiClient.LevelV1().PodMetrics().Create(currentPodMetric)
				if err != nil {
					klog.Errorln(err)
				}
			} else {
				_, err = nc.KetiClient.LevelV1().PodMetrics().Update(currentPodMetric)
				if err != nil {
					klog.Errorln(err)
				}
			}

		}
		
		prevPods = collection.Metricsbatch.Pods
		time.Sleep(time.Second * 5)
	}

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
