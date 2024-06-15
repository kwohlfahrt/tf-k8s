terraform {
  required_providers {
    k8scrd = {
      source = "github.com/kwohlfahrt/k8scrd"
    }
  }
}

provider "k8scrd" {
  kubeconfig = file("./kubeconfig.yaml")
}

data "k8scrd_foo_example_com_v1" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}

resource "k8scrd_foo_example_com_v1" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
  }
  spec = {
    foo = "bar"
  }
}

resource "k8scrd_bar_example_com_v1" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
  }
  spec = {
    bar = "bar"
  }
}

output "cert_spec" {
  value = data.k8scrd_foo_example_com_v1.foo.spec
}
