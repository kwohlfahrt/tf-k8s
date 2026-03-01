variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_middleware_traefik_io_v1alpha1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    spec     = { forward_auth = { address = "http://bar.example.com/auth" } }
  }
}

import {
  to = k8scrd_middleware_traefik_io_v1alpha1.bar
  id = "kubectl:default/bar"
}
