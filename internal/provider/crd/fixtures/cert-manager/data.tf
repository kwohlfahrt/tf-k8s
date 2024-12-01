variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_certificate_certmanager_io_v1" "foo" {
  metadata = { name = "foo", namespace = "default" }
}

data "k8scrd_issuer_certmanager_io_v1" "foo" {
  metadata = { name = "foo", namespace = "default" }
}
