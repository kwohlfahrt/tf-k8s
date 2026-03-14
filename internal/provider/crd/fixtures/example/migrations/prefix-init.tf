variable "kubeconfig" {
  type      = string
  sensitive = true
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
    spec     = { foo = "prefix" }
  }
}
