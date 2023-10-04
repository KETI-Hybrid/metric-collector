#!/bin/bash
docker_id="ketidevit2"
controller_name="hybrid.metric-collector"
docker build -t $docker_id/$controller_name:latest /root/workspace/hth/dev/metric-collector/build && \
docker push $docker_id/$controller_name:latest


#0.0.2 => 운용중인 metric collector member
#0.0.3 => test 용 버전 (제병)