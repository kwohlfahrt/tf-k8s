variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_ippool_projectcalico_org_v3" "baz" {
  manifest = {
    metadata = { name = "baz" }
    spec     = { cidr = "192.0.2.0/24", nat_outgoing = false }
  }
}

import {
  to = k8scrd_ippool_projectcalico_org_v3.baz
  id = "kubectl:baz"
}
