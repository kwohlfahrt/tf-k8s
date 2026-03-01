variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_middleware_traefik_io_v1alpha1" "baz" {
  manifest = {
    metadata = { name = "baz", namespace = "default" }
    spec     = { forward_auth = { address = "http://baz.example.com/auth" } }
  }
}
