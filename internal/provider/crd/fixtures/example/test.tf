variable "kubeconfig" {
  type      = string
  sensitive = true
}

variable "update" {
  type    = bool
  default = false
}

terraform {
  required_providers {
    k8s = {
      source = "registry.terraform.io/hashicorp/k8s"
    }
  }
}

provider "k8s" {
  kubeconfig = var.kubeconfig
}

data "k8s_foo_example_com_v1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
}

resource "k8s_foo_example_com_v1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    spec     = { foo = "bar" }
  }
}

resource "k8s_bar_example_com_v1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    spec     = { bar = var.update ? "barbar" : "bar" }
  }
}

resource "k8s_foo_example_com_v1" "baz" {
  manifest = {
    metadata = { name = "baz", namespace = "default" }
    spec     = { foo = var.update ? "bazbaz" : "baz" }
  }
}

import {
  to = k8s_foo_example_com_v1.baz
  id = "kubectl:default/baz"
}

output "foo" {
  value = provider::k8s::parse_foo_example_com_v1({
    apiVersion = "example.com/v1"
    kind = "Foo"
    metadata = {
      name      = "bar"
      namespace = "default"
    }
    spec = { foo = "bar" }
  })
}
