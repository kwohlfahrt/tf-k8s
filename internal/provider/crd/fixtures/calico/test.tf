variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_ippool_projectcalico_org_v3" "foo" {
  manifest = { metadata = { name = "foo" } }
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

output "ippool" {
  value = provider::k8scrd::parse_ippool_crd_projectcalico_org_v1({
    apiVersion = "v1"
    kind       = "IPPool"
    metadata   = { name = "bar", namespace = "default" }
    spec       = { cidr = "198.51.100.0/30" }
  })
}
