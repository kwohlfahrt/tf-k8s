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
