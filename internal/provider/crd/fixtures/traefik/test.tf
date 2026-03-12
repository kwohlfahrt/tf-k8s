variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_middleware_traefik_io_v1alpha1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
}

resource "k8scrd_middleware_traefik_io_v1alpha1" "baz" {
  manifest = {
    metadata = { name = "baz", namespace = "default" }
    spec     = { forward_auth = { address = "http://baz.example.com/auth" } }
  }
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

output "foo" {
  value = provider::k8scrd::parse_middleware_traefik_io_v1alpha1({
    apiVersion = "traefik.io/v1alpha1"
    kind = "Middleware"
    metadata = { name = "forwardauth", namespace = "default" }
    spec = { forwardAuth = { address = "http://example.com/auth" }}
  })
}
