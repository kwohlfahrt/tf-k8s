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
