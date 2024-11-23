variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

output "ippool" {
  value = provider::k8scrd::parse_ippool_crd_projectcalico_org_v1({
    apiVersion = "v1"
    kind       = "IPPool"
    metadata   = { name = "bar", namespace = "default" }
    spec       = { cidr = "198.51.100.0/30" }
  })
}
