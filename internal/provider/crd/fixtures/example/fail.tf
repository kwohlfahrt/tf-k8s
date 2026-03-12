variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8s" {
  kubeconfig = var.kubeconfig
}

resource "k8s_foo_example_com_v1" "qux-0" {
  manifest = {
    metadata = { name = "qux", namespace = "default" }
    spec     = { foo = "qux" }
  }
}

resource "k8s_foo_example_com_v1" "qux-1" {
  manifest = {
    metadata = { name = "qux", namespace = "default" }
    spec     = { foo = "qux" }
  }
}
