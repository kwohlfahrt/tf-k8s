#!/usr/bin/env bash -euo pipefail

CRD_FILE=cert-manager.crds.yaml
curl --location https://github.com/cert-manager/cert-manager/releases/download/v1.14.5/cert-manager.crds.yaml \
  | yq 'select(.metadata.name == "certificates.cert-manager.io")' > $CRD_FILE

kubectl apply --server-side -f $CRD_FILE
kubectl apply --server-side -f - <<EOF
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
