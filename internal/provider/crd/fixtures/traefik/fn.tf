variable "kubeconfig" {
  type      = string
  sensitive = true
}

provider "k8scrd" {
  kubeconfig = var.kubeconfig
}

output "foo" {
  value = provider::k8scrd::parse_middleware_traefik_io_v1alpha1({
    apiVersion = "traefik.io/v1alpha1"
    kind = "Middleware"
    metadata = { name = "forwardauth", namespace = "default" }
    spec = { forwardAuth = { address = "http://example.com/auth" }}
  })
}
