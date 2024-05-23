terraform {
  required_providers {
    tfcrd = {
      source = "github.com/kwohlfahrt/k8scrd"
    }
  }
}

provider "tfcrd" {
  kubeconfig = file("./kubeconfig.yaml")
}

data "tfcrd_certificate" "cert" {
  metadata = {
    name      = "cert"
    namespace = "default"
  }
}

output "dns_names" {
  value = data.tfcrd_certificate.cert.spec.dns_names
}
