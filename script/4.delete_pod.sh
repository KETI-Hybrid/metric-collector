#!/bin/bash

# kubectl delete -f deploy/ --context arn:aws:eks:us-east-2:741566967679:cluster/eks-cluster
# kubectl delete -f deploy/ --context master2
#kubectl delete -f deploy/ --context cluster3
#kubectl delete -f deploy/ --context cluster4
#kubectl delete -f deploy/ --context cluster5
#kubectl delete -f deploy/ --context aks-test
# kubectl delete -f deploy/ --context uni-master
#kubectl delete -f deploy/ --context gke_keti-container_us-central1-a_gke-cluster
#kubectl delete -f deploy/ --context eks-cluster1


# kubectl delete -f deploy/operator/operator-eks-master.yaml --context arn:aws:eks:us-east-2:741566967679:cluster/eks-cluster
# kubectl delete -f deploy/operator/operator-cluster2.yaml --context master2
#kubectl delete -f deploy/operator/operator-cluster3.yaml --context cluster3
#kubectl delete -f deploy/operator/operator-cluster4.yaml --context cluster4
#kubectl delete -f deploy/operator/operator-cluster5.yaml --context cluster5
#kubectl delete -f deploy/operator/operator-aks-cluster.yaml --context aks-test
# kubectl delete -f deploy/operator/operator-uni-cluster.yaml --context uni-master
#kubectl delete -f deploy/operator/operator-gke-cluster.yaml --context gke_keti-container_us-central1-a_gke-cluster
#kubectl delete -f deploy/operator/operator-eks-cluster1.yaml --context eks-cluster1



kubectl delete -f /root/workspace/hth/dev/metric-collector/deploy
