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

data "k8scrd_configmap_v1" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}


data "k8scrd_deployment_apps_v1" "foo" {
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

resource "k8scrd_configmap_v1" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
  }
  data = {
    "foo.txt" = "hello, world!"
  }
}

resource "k8scrd_deployment_apps_v1" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
    labels    = { app = "bar" }
  }
  spec = {
    replicas = 0
    selector = { match_labels = { app = "bar" } }
    template = {
      metadata = { labels = { app = "bar" } }
      spec = {
        containers = [{ name = "bar", image = "busybox" }]
      }
    }
  }
}
