variable "kubeconfig" {
  type      = string
  sensitive = true
}

variable "update" {
  type    = bool
  default = false
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
    spec     = { bar = var.update ? "barbar" : "bar" }
  }
}

resource "k8scrd_foo_example_com_v1" "baz" {
  manifest = {
    metadata = { name = "baz", namespace = "default" }
    spec     = { foo = "baz" }
  }
}

import {
  to = k8scrd_foo_example_com_v1.baz
  id = "kubectl:default/baz"
}
