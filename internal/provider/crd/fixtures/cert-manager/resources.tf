variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_certificate_certmanager_io_v1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    spec = {
      issuer_ref   = { kind = "ClusterIssuer", name = "self-signed" }
      dns_names    = ["example.org"]
      duration     = "2160h"
      renew_before = "360h"
      secret_name  = "example-org"
    }
  }
}
