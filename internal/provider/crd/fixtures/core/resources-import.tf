variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_deployment_apps_v1" "baz" {
  manifest = {
    metadata = {
      name      = "baz"
      namespace = "default"
      labels    = { app = "baz" }
    }
    spec = {
      replicas = 0
      selector = { match_labels = { app = "baz" } }
      template = {
        metadata = { labels = { app = "baz" } }
        spec = {
          containers = [{
            name  = "baz"
            image = "busybox"
          }]
        }
      }
    }
  }
}

import {
  to = k8scrd_deployment_apps_v1.baz
  id = "kubectl:default/baz"
}

resource "k8scrd_namespace_v1" "baz" {
  manifest = { metadata = { name = "baz" } }
}

import {
  to = k8scrd_namespace_v1.baz
  id = "kubectl:baz"
}
