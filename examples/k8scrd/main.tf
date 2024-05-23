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

data "tfcrd_certificate" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}

resource "tfcrd_certificate" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
  }
  spec = {
    dns_names = ["bar.example.com"]
    issuer_ref = {
      group = "cert-manager.io"
      kind  = "ClusterIssuer"
      name  = "production"
    }
    secret_name = "bar"
  }
}

output "cert_spec" {
  value = data.tfcrd_certificate.foo.spec
}
