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
    spec     = { cidr = "203.0.113.0/24" }
  }
}
