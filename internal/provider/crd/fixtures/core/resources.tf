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
        volumes = []
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

resource "k8scrd_namespace_v1" "bar" {
  metadata = {
    name   = "bar"
    labels = { "bar" = "bar" }
  }
}

resource "k8scrd_clusterrole_rbac_authorization_k8s_io_v1" "bar" {
  metadata = { name = "bar" }
  rules = [
    { api_groups = [""], resources = ["pods"], verbs = ["get", "list", "watch"] },
  ]
}
