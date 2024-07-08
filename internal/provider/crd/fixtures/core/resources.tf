variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
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
    strategy = {
      type = "RollingUpdate"
      rolling_update = {
        max_unavailable = 1
      }
    }
    template = {
      metadata = { labels = { app = "bar" } }
      spec = {
        containers = [{
          name  = "foo"
          image = "busybox"
        }]
      }
    }
  }
}

resource "k8scrd_configmap_v1" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
  }

  data = { "foo.txt" = "bar" }
}

resource "k8scrd_deployment_apps_v1" "baz" {
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

import {
  to = k8scrd_deployment_apps_v1.baz
  id = "default/baz"
}

resource "k8scrd_namespace_v1" "bar" {
  metadata = {
    name   = "bar"
    labels = { "bar" = "bar" }
  }
}

resource "k8scrd_namespace_v1" "baz" {
  metadata = { name = "baz" }
}

import {
  to = k8scrd_namespace_v1.baz
  id = "baz"
}

resource "k8scrd_clusterrole_rbac_authorization_k8s_io_v1" "bar" {
  metadata = { name = "bar" }
  rules = [
    { api_groups = [""], resources = ["pods"], verbs = ["get", "list", "watch"] },
  ]
}
