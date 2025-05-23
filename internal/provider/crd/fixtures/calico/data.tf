variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_ippool_crd_projectcalico_org_v1" "foo" {
  manifest = { metadata = { name = "foo" } }
}
