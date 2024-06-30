variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_foo_example_com_v1" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}
