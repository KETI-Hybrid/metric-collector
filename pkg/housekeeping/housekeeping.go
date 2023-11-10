package housekeeping

import (
	"context"
	"time"

	keticlient "github.com/KETI-Hybrid/keti-controller/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type HouseKeeper struct {
	KetiClient *keticlient.ClientSet
	KubeClient *kubernetes.Clientset
}

func NewHouseKeeper(keticlient *keticlient.ClientSet, kubeclient *kubernetes.Clientset) *HouseKeeper {
	return &HouseKeeper{
		KetiClient: keticlient,
		KubeClient: kubeclient,
	}

}

func (hk *HouseKeeper) NodeKeeping() {
	for {
		nodeMap := make(map[string]bool)
		nodes, err := hk.KubeClient.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			klog.Errorln(err)
		}
		for _, node := range nodes.Items {
			nodeMap[node.Name] = true
		}
		nodeMetrics, err := hk.KetiClient.LevelV1().NodeMetrics().List(metav1.ListOptions{})
		if err != nil {
			klog.Errorln(err)
		}
		for _, nodeMetric := range nodeMetrics.Items {
			if !nodeMap[nodeMetric.Name] {
				err := hk.KetiClient.LevelV1().NodeMetrics().Delete(nodeMetric.Name, &metav1.DeleteOptions{})
				if err != nil {
					klog.Errorln(err)
				}
			}
		}
		time.Sleep(time.Second * 10)
	}
}

func (hk *HouseKeeper) PodKeeping() {
	for {
		podMap := make(map[string]bool)
		pods, err := hk.KubeClient.CoreV1().Pods(corev1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			klog.Errorln(err)
		}
		for _, pod := range pods.Items {
			if pod.Namespace == "cdi" || pod.Namespace == "keti-controller-system" || pod.Namespace == "keti-system" || pod.Namespace == "kube-flannel" || pod.Namespace == "kube-node-lease" || pod.Namespace == "kube-public" || pod.Namespace == "kube-system" || pod.Namespace == "kubevirt" {
				continue
			} else {
				podMap[pod.Name] = true
			}
		}
		podMetrics, err := hk.KetiClient.LevelV1().PodMetrics().List(metav1.ListOptions{})
		if err != nil {
			klog.Errorln(err)
		}
		for _, podMetric := range podMetrics.Items {
			if !podMap[podMetric.Name] {
				err := hk.KetiClient.LevelV1().PodMetrics().Delete(podMetric.Name, &metav1.DeleteOptions{})
				if err != nil {
					klog.Errorln(err)
				}
			}
		}
		time.Sleep(time.Second * 10)
	}
}
