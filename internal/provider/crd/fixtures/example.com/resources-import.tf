variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_foo_example_com_v1" "baz" {
  metadata = {
    name      = "baz"
    namespace = "default"
  }
  spec = { foo = "baz" }
}

import {
  to = k8scrd_foo_example_com_v1.baz
  id = "kubectl:default/baz"
}
