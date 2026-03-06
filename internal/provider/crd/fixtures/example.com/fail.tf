variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_foo_example_com_v1" "qux-0" {
  manifest = {
    metadata = { name = "qux", namespace = "default" }
    spec     = { foo = "qux" }
  }
}

resource "k8scrd_foo_example_com_v1" "qux-1" {
  manifest = {
    metadata = { name = "qux", namespace = "default" }
    spec     = { foo = "qux" }
  }
}
