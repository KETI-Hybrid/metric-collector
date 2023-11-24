package crd

import (
	"context"
	"fmt"
	"os"
	"time"
	authv1 "github.com/KETI-Hybrid/keti-controller/apis/auth/v1"
	cloudv1 "github.com/KETI-Hybrid/keti-controller/apis/cloud/v1"
	levelv1 "github.com/KETI-Hybrid/keti-controller/apis/level/v1"
	resourcev1 "github.com/KETI-Hybrid/keti-controller/apis/resource/v1"
	keticlient "github.com/KETI-Hybrid/keti-controller/client"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func NewClient() (*keticlient.ClientSet, error) {
	err := authv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Errorln(err)
	}
	err = cloudv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Errorln(err)
	}
	err = resourcev1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Errorln(err)
	}
	err = levelv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Errorln(err)
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorln(err)
	}
	return keticlient.NewForConfig(config)
}
