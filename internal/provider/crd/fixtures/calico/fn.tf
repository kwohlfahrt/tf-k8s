variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

output "ippool" {
  value = provider::k8scrd::parse_ippool_projectcalico_org_v3({
    apiVersion = "v3"
    kind       = "IPPool"
    metadata   = { name = "bar", namespace = "default" }
    spec       = { cidr = "198.51.100.0/30" }
  })
}
