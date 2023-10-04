#/bin/bash
NS=keti-system
NAME=$(kubectl get pod -n $NS | grep -E 'metric-collector' | awk '{print $1}')

kubectl logs -f -n $NS $NAME

