variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_ippool_projectcalico_org_v3" "bar" {
  manifest = {
    metadata = { name = "bar" }
    spec     = { cidr = "198.51.100.128/26" }
  }
}

resource "k8scrd_ippool_projectcalico_org_v3" "baz" {
  manifest = {
    metadata = { name = "baz" }
    spec     = { cidr = "198.51.100.64/26", nat_outgoing = true }
  }
}

import {
  to = k8scrd_ippool_projectcalico_org_v3.baz
  id = "kubectl:baz"
}
