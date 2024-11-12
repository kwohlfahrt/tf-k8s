variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

locals {
  pod = provider::k8scrd::parse_foo_example_com_v1({
    apiVersion = "example.com/v1"
    kind = "Foo"
    metadata = {
      name      = "bar"
      namespace = "default"
    }
    spec = { foo = "bar" }
  })
}
