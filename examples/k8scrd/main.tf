terraform {
  required_providers {
    tfcrd = {
      source = "github.com/kwohlfahrt/k8scrd"
    }
  }
}

provider "tfcrd" {}
