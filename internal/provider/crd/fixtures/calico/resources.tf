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
