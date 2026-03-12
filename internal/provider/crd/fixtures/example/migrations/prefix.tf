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

resource "k8s_foo_example_com_v1" "prefix" {
  manifest = {
    metadata = { name = "prefix", namespace = "default" }
    spec     = { foo = var.update ? "prefix" : "prefix-pos" }
  }
}

moved {
  from = k8scrd_foo_example_com_v1.prefix
  to   = k8s_foo_example_com_v1.prefix
}
