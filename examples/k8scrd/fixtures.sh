#!/usr/bin/env bash -euo pipefail

SRC_DIR=$(dirname ${BASH_SOURCE})/../..
kubectl apply --server-side -f ${SRC_DIR}/internal/test.crds.yaml
kubectl apply --server-side -f - <<EOF
apiVersion: example.com/v1
kind: Foo
metadata:
  name: foo
spec:
  foo: foo
EOF
