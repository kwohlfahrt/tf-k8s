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

data "k8scrd_foo_example_com" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}

resource "k8scrd_foo_example_com" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
  }
  spec = {
    foo = "bar"
  }
}

output "cert_spec" {
  value = data.k8scrd_foo_example_com.foo.spec
}
