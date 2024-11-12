variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

output "pod" {
  value = provider::k8scrd::parse_pod_v1({
    apiVersion = "v1"
    kind       = "Pod"
    metadata = {
      name      = "bar"
      namespace = "default"
    }
    spec = { containers = [{ name : "ubuntu", image : "ubuntu:22.04" }] }
  })
}
