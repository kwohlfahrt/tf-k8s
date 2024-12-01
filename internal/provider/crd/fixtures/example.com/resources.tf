variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_foo_example_com_v1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    spec     = { foo = "bar" }
  }
}

resource "k8scrd_bar_example_com_v1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    spec     = { bar = "bar" }
  }
}
