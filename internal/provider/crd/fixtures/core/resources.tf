variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

resource "k8scrd_deployment_apps_v1" "bar" {
  manifest = {
    metadata = {
      name      = "bar"
      namespace = "default"
      labels    = { app = "bar" }
    }
    spec = {
      replicas = 0
      min_ready_seconds = 0
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
          containers = [
            { name = "foo", image = "busybox" },
            { name = "ubuntu", image = "ubuntu:22.04", liveness_probe = { http_get = { port = "healthz" } } },
          ]
          volumes = []
        }
      }
    }
  }
}

resource "k8scrd_configmap_v1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    data     = { "foo.txt" = "bar" }
  }
}

resource "k8scrd_namespace_v1" "bar" {
  manifest = {
    metadata = {
      name   = "bar"
      labels = { "bar" = "bar" }
    }
  }
}

resource "k8scrd_clusterrole_rbac_authorization_k8s_io_v1" "bar" {
  manifest = {
    metadata = { name = "bar" }
    rules = [
      { api_groups = [""], resources = ["pods"], verbs = ["get", "list", "watch"] },
    ]
  }
}

resource "k8scrd_priorityclass_scheduling_k8s_io_v1" "bar" {
  manifest = {
    metadata = { name = "bar" }
    preemption_policy = "Never"
    value = -1
  }
}

resource "k8scrd_gatewayclass_gateway_networking_k8s_io_v1" "bar" {
  manifest = {
    metadata = { name = "bar" }
    spec = { controller_name = "example.com/foo" }
  }
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

resource "k8scrd_clusterrolebinding_rbac_authorization_k8s_io_v1" "baz" {
  manifest = {
    metadata = { name = "baz" }
    role_ref = {
      api_group = "rbac.authorization.k8s.io"
      kind      = "ClusterRole"
      name      = "system:node"
    }
  }
}

import {
  to = k8scrd_clusterrolebinding_rbac_authorization_k8s_io_v1.baz
  id = "kubectl:baz"
}
