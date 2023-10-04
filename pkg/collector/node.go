package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	metricCore "metric-collector/pkg/api/grpc"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

type Provider struct {
	metricCore.UnsafeMetricGRPCServer
	PodPromRegistry  *prometheus.Registry
	NodePromRegistry *prometheus.Registry
}

func (p *Provider) GetNode(ctx context.Context, req *metricCore.Request) (*metricCore.Response, error) {
	klog.Infoln("request recieve")
	metricFamily, err := p.NodePromRegistry.Gather()
	if err != nil {
		klog.Errorln(err)
		return nil, err
	}
	metricMap := make(map[string]*metricCore.MetricFamily)
	for _, mf := range metricFamily {
		tmp_mf := &metricCore.MetricFamily{}

		jsonByte, err := json.Marshal(mf)
		if err != nil {
			klog.Errorln(err)
			return nil, err
		}
		err = json.Unmarshal(jsonByte, tmp_mf)
		if err != nil {
			klog.Errorln(err)
			return nil, err
		}

		metricMap[*mf.Name] = tmp_mf
	}
	klog.Infoln("request process End")
	return &metricCore.Response{Message: metricMap}, nil
}

func (p *Provider) GetPod(ctx context.Context, req *metricCore.Request) (*metricCore.Response, error) {
	klog.Infoln("request recieve")
	metricFamily, err := p.PodPromRegistry.Gather()
	if err != nil {
		klog.Errorln(err)
		return nil, err
	}
	metricMap := make(map[string]*metricCore.MetricFamily)
	for _, mf := range metricFamily {
		tmp_mf := &metricCore.MetricFamily{}

		jsonByte, err := json.Marshal(mf)
		if err != nil {
			klog.Errorln(err)
			return nil, err
		}
		err = json.Unmarshal(jsonByte, tmp_mf)
		if err != nil {
			klog.Errorln(err)
			return nil, err
		}

		metricMap[*mf.Name] = tmp_mf
	}
	klog.Infoln("request process End")
	return &metricCore.Response{Message: metricMap}, nil
}

func RunCollectorServer(podreg *prometheus.Registry, nodereg *prometheus.Registry) {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}
	nodeServer := grpc.NewServer()
	metricCore.RegisterMetricGRPCServer(nodeServer, &Provider{
		PodPromRegistry:  podreg,
		NodePromRegistry: nodereg,
	})
	fmt.Println("node server started...")
	if err := nodeServer.Serve(lis); err != nil {
		klog.Fatalf("failed to serve: %v", err)
	}
}
