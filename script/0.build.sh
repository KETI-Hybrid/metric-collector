#!/bin/bash
controller_name="hybrid.metric-collector"
export GO111MODULE=on
go mod vendor
go build -o build/_output/bin/$controller_name -mod=vendor /root/workspace/hth/dev/metric-collector/cmd/main.go


#0.0.2 => 운용중인 metric collector member
#0.0.3 => test 용 버전 (제병)