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
