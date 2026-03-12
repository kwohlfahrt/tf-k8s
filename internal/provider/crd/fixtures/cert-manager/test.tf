variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_certificate_certmanager_io_v1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
}

data "k8scrd_issuer_certmanager_io_v1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
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

resource "k8scrd_certificate_certmanager_io_v1" "baz" {
  manifest = {
    metadata = { name = "baz", namespace = "default" }
    spec = {
      issuer_ref   = { kind = "ClusterIssuer", name = "self-signed" }
      dns_names    = ["example.org"]
      duration     = "2160h"
      renew_before = "360h"
      secret_name  = "example-org"
    }
  }
}

import {
  to = k8scrd_certificate_certmanager_io_v1.baz
  id = "kubectl:default/baz"
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
