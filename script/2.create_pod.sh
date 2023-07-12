#!/bin/bash

# kubectl create ns hcp --context arn:aws:eks:us-east-2:741566967679:cluster/eks-cluster
# kubectl create ns hcp --context aks-test
# kubectl create ns hcp --context uni-master
# kubectl create ns hcp --context master2
# kubectl create ns hcp --context eks-keti-cluster1

# kubectl create -f deploy/ --context arn:aws:eks:us-east-2:741566967679:cluster/eks-cluster
# kubectl create -f deploy/ --context master2
# kubectl create -f deploy/ --context aks-test
# kubectl create -f deploy/ --context uni-master
# kubectl create -f deploy/ --context gke_keti-container_us-central1-a_gke-cluster
# kubectl create -f deploy/ --context eks-keti-cluster1
kubectl create -f /root/workspace/hth/dev/metric-collector/deploy/

# kubectl create -f deploy/operator/operator-eks-master.yaml --context arn:aws:eks:us-east-2:741566967679:cluster/eks-cluster
# kubectl create -f deploy/operator/operator-cluster2.yaml --context master2

# kubectl create -f deploy/operator/operator-aks-cluster.yaml --context aks-cluster-admin
# kubectl create -f deploy/operator/operator-aks-cluster.yaml --context aks-test
# kubectl create -f deploy/operator/operator-uni-cluster.yaml --context uni-master
# kubectl create -f deploy/operator/operator-gke-cluster.yaml --context gke_keti-container_us-central1-a_gke-cluster
# kubectl create -f deploy/operator/operator-eks-master.yaml --context eks-keti-cluster1
#kubectl create -f deploy/operator/operator-eks-master.yaml --context eks-keti-cluster2

