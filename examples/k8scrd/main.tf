terraform {
  required_providers {
    k8scrd = {
      source = "github.com/kwohlfahrt/k8scrd"
    }
  }
}

provider "k8scrd" {
  kubeconfig = file("./kubeconfig.yaml")
}

data "k8scrd_certificate" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}

resource "k8scrd_certificate" "bar" {
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
  value = data.k8scrd_certificate.foo.spec
}
