variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

data "k8scrd_deployment_apps_v1" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}

data "k8scrd_configmap_v1" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}
