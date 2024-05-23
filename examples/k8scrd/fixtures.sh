#!/usr/bin/env bash -euo pipefail

kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.5/cert-manager.crds.yaml
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: foo
spec:
  dnsNames:
  - foo.example.com
  issuerRef:
    group: cert-manager.io
    kind: ClusterIssuer
    name: production
  secretName: foo
EOF
