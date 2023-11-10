package image

import (
	"context"
	"encoding/json"
	"time"

	keticlient "github.com/KETI-Hybrid/keti-controller/client"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type ImageData struct {
	ID string `json:"id"`
}

type ImageManager struct {
	KetiClient *keticlient.ClientSet
	KubeClient *kubernetes.Clientset
	NodeName   string
}

func NewImageManager(keticlient *keticlient.ClientSet, kubeclient *kubernetes.Clientset, nodeName string) *ImageManager {
	return &ImageManager{
		KetiClient: keticlient,
		KubeClient: kubeclient,
		NodeName:   nodeName,
	}
}

func (im *ImageManager) ImageWatcher() {
	// Docker 클라이언트 초기화
	cli, err := client.NewClientWithOpts(client.WithVersion("1.40"))
	if err != nil {
		panic(err)
	}

	for {
		isFirst := true
		// 이미지 목록 가져오기
		images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
		if err != nil {
			panic(err)
		}
		dataList := make([]ImageData, len(images))

		cms, err := im.KubeClient.CoreV1().ConfigMaps("keti-system").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			klog.Errorln(err)
		}
		for _, image := range images {
			if len(image.RepoTags) > 0 {
				dataList = append(dataList, ImageData{ID: image.RepoTags[0]})
			}
		}
		jsonByte, err := json.MarshalIndent(dataList, "", "  ")
		if err != nil {
			klog.Errorln(err)
		}
		jsonData := string(jsonByte)

		for _, configmap := range cms.Items {
			if configmap.Name == "imagelist" {
				isFirst = false
				configmap.Data[im.NodeName+".json"] = jsonData
				_, err = im.KubeClient.CoreV1().ConfigMaps("keti-system").Update(context.Background(), &configmap, metav1.UpdateOptions{})
				if err != nil {
					klog.Errorln(err)
				}
				break
			}
		}

		if isFirst {
			cm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "imagelist",
					Namespace: "keti-system",
				},
				Data: make(map[string]string),
			}
			cm.Data[im.NodeName+".json"] = jsonData
			_, err = im.KubeClient.CoreV1().ConfigMaps("keti-system").Create(context.Background(), cm, metav1.CreateOptions{})
			if err != nil {
				klog.Errorln(err)
			}
		}

		time.Sleep(30 * time.Second)
	}
}
