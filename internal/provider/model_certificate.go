package provider

import (
	_ "embed"

	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

var certificateGvr = runtimeschema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}

type objectMeta struct {
	Name      string `tfsdk:"name"`
	Namespace string `tfsdk:"namespace"`
}

//go:embed cert-manager.crds.yaml
var schemaBytes []byte
