#/bin/bash
NODE=$1
NS=keti-system
NAME=$(kubectl get pod -n $NS -o wide | grep -E 'metric-collector' | grep -E $NODE | awk '{print $1}')

# echo $NS
# echo $NAME

kubectl logs -f -n $NS $NAME

