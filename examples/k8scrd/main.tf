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

data "k8scrd_certificate" "foo" {
  metadata = {
    name      = "foo"
    namespace = "default"
  }
}

resource "k8scrd_certificate" "bar" {
  metadata = {
    name      = "bar"
    namespace = "default"
  }
  spec = {
    dns_names = ["bar.example.com"]
    issuer_ref = {
      group = "cert-manager.io"
      kind  = "ClusterIssuer"
      name  = "production"
    }
    secret_name = "bar"
    # TODO: Make these all optional
    additional_output_formats = []
    common_name               = "bar.example.com"
    duration                  = "1d"
    email_addresses           = []
    encode_usages_in_request  = false
    ip_addresses              = []
    is_ca                     = false
    keystores = {
      jks    = { create = false, password_secret_ref = { key = "", name = "" } }
      pkcs12 = { create = false, profile = "", password_secret_ref = { key = "", name = "" } }
    }
    literal_subject = ""
    name_constraints = {
      critical  = false
      permitted = { dns_domains = [], uri_domains = [], email_addresses = [], ip_ranges = [] }
      excluded  = { dns_domains = [], uri_domains = [], email_addresses = [], ip_ranges = [] }
    }
    other_names = []
    private_key = {
      algorithm       = ""
      encoding        = ""
      rotation_policy = ""
      size            = 0
    }
    renew_before           = ""
    revision_history_limit = 0
    secret_template        = { annotations = {}, labels = {} }
    uris                   = []
    subject = {
      countries            = []
      localities           = []
      organizational_units = []
      organizations        = []
      postal_codes         = []
      provinces            = []
      serial_number        = ""
      street_addresses     = []
    }
    usages = []
  }
}

output "cert_spec" {
  value = data.k8scrd_certificate.foo.spec
}
