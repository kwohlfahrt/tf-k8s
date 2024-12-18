variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_deployment_apps_v1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
}

data "k8scrd_configmap_v1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
}

data "k8scrd_namespace_v1" "foo" {
  manifest = { metadata = { name = "foo" } }
}
