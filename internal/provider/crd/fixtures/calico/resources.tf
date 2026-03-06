variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_ippool_crd_projectcalico_org_v1" "bar" {
  manifest = {
    metadata = { name = "bar" }
    spec     = { cidr = "198.51.100.8/30" }
  }
}

resource "k8scrd_ippool_projectcalico_org_v3" "bar" {
  manifest = {
    metadata = { name = "qux" }
    spec     = { cidr = "192.0.2.0/24" }
  }
}

resource "k8scrd_ippool_crd_projectcalico_org_v1" "baz" {
  manifest = {
    metadata = { name = "baz" }
    spec     = { cidr = "198.51.100.4/30", nat_outgoing = false }
  }
}

import {
  to = k8scrd_ippool_crd_projectcalico_org_v1.baz
  id = "kubectl:baz"
}
