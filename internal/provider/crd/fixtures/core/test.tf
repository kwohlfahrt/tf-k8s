variable "kubeconfig" {
  type      = string
  sensitive = true
}

terraform {
  required_providers {
    k8s = {
      source = "registry.terraform.io/hashicorp/k8s"
    }
  }
}

provider "k8s" {
  kubeconfig = var.kubeconfig
}

data "k8s_deployment_apps_v1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
}

data "k8s_configmap_v1" "foo" {
  manifest = { metadata = { name = "foo", namespace = "default" } }
}

data "k8s_namespace_v1" "foo" {
  manifest = { metadata = { name = "foo" } }
}

resource "k8s_deployment_apps_v1" "bar" {
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
          # k8s doesn't include an empty volumes field in `managedFields`. Test
          # that we handle this properly.
          volumes = []
        }
      }
    }
  }
}

resource "k8s_configmap_v1" "bar" {
  manifest = {
    metadata = { name = "bar", namespace = "default" }
    data     = { "foo.txt" = "bar" }
  }
}

resource "k8s_namespace_v1" "bar" {
  manifest = {
    metadata = {
      name   = "bar"
      labels = { "bar" = "bar" }
    }
  }
}

resource "k8s_clusterrole_rbac_authorization_k8s_io_v1" "bar" {
  manifest = {
    metadata = { name = "bar" }
    rules = [
      { api_groups = [""], resources = ["pods"], verbs = ["get", "list", "watch"] },
    ]
  }
}

resource "k8s_priorityclass_scheduling_k8s_io_v1" "bar" {
  manifest = {
    metadata = { name = "bar" }
    preemption_policy = "Never"
    value = -1
  }
}

resource "k8s_gatewayclass_gateway_networking_k8s_io_v1" "bar" {
  manifest = {
    metadata = { name = "bar" }
    spec = { controller_name = "example.com/foo" }
  }
}

resource "k8s_deployment_apps_v1" "baz" {
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
  to = k8s_deployment_apps_v1.baz
  id = "kubectl:default/baz"
}

resource "k8s_clusterrolebinding_rbac_authorization_k8s_io_v1" "baz" {
  manifest = {
    metadata = { name = "baz" }
    role_ref = {
      api_group = "rbac.authorization.k8s.io"
      kind      = "ClusterRole"
      name      = "system:node"
    }
    subjects = []
  }
  # Required due to migration not working for some empty fields. See
  # kubernetes/kubernetes#128924.
  force_conflicts = true
}

import {
  to = k8s_clusterrolebinding_rbac_authorization_k8s_io_v1.baz
  id = "kubectl:baz"
}

output "pod" {
  value = provider::k8s::parse_pod_v1({
    apiVersion = "v1"
    kind       = "Pod"
    metadata   = { name = "bar", namespace = "default" }
    spec = { containers = [
      { name = "ubuntu", image = "ubuntu:22.04", livenessProbe = { httpGet = { port = "healthz" } } },
      { name = "also-ubuntu", image = "ubuntu:22.04" }
    ] }
  })
}

output "configmap" {
  value = provider::k8s::parse_configmap_v1({
    apiVersion = "v1"
    kind       = "ConfigMap"
    metadata   = { name = "bar", namespace = "default" }
    data       = null // Sometimes seen in manifests with empty data
  })
}
