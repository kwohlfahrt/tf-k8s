#!/usr/bin/env bash -euo pipefail

SRC_DIR=$(dirname ${BASH_SOURCE})/../..
kubectl apply --server-side -f ${SRC_DIR}/internal/provider/crd/test.crds.yaml
kubectl apply --server-side -f - <<EOF
apiVersion: example.com/v1
kind: Foo
metadata:
  name: foo
spec:
  foo: foo
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: foo
  labels:
    app: foo
spec:
  replicas: 0
  selector:
    matchLabels:
      app: foo
  template:
    metadata:
      labels:
        app: foo
    spec:
      containers:
      - name: foo
        image: busybox
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  foo.txt: |
    hello, world!
EOF
