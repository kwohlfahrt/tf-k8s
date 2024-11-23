variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

output "certificate" {
  value = provider::k8scrd::parse_certificate_certmanager_io_v1({
    apiVersion = "cert-manager.io/v1"
    kind       = "Certificate"
    metadata   = { name = "bar", namespace = "default" }
    spec = {
      issuerRef   = { kind = "ClusterIssuer", name = "self-signed" }
      dnsNames    = ["example.org"]
      duration    = "2160h"
      renewBefore = "360h"
      secretName  = "example-org"
    }
  })
}
