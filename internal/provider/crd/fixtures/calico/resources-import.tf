variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_ippool_crd_projectcalico_org_v1" "baz" {
  metadata = { name = "baz" }
  spec = {
    cidr         = "198.51.100.4/30"
    nat_outgoing = false
  }
}

import {
  to = k8scrd_ippool_crd_projectcalico_org_v1.baz
  id = "kubectl:baz"
}
