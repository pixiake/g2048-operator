# G2048 Operator
## Overview

The G2048 Operator provides native deployment and management of
[the game 2048](https://github.com/gabrielecirulli/2048).

## Usage

* create 2048 Operator
```
kubectl apply -f https://raw.githubusercontent.com/pixiake/g2048-operator/master/manifests/2048-operator.yaml
```
* create 2046 custome source
```
cat <<EOF | kubectl apply -f -
apiVersion: game.example.com/v1alpha1
kind: G2048
metadata:
  name: g2048-sample
  namespace: default
spec:
  serviceType: NodePort
  size: 2
EOF
```

> `serviceType` defineds the type of service, can be "NodePort", "ClusterIP", "LoadBalancer".

> `size` is the replicas of the g2048 deployment.