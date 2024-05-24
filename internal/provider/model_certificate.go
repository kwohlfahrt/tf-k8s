package provider

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

type certificateModel struct {
	Metadata *certificateMetadata `tfsdk:"metadata"`
	Spec     *certificateSpec     `tfsdk:"spec"`
}

type certificateMetadata struct {
	Name      string `tfsdk:"name"`
	Namespace string `tfsdk:"namespace"`
}

type certificateSpec struct {
	DnsNames   []string  `tfsdk:"dns_names"`
	IssuerRef  issuerRef `tfsdk:"issuer_ref"`
	SecretName string    `tfsdk:"secret_name"`
}

type issuerRef struct {
	Group string `tfsdk:"group"`
	Kind  string `tfsdk:"kind"`
	Name  string `tfsdk:"name"`
}

var certificateGvr = runtimeschema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}

func dumpCertificate(data *certificateModel) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      data.Metadata.Name,
				"namespace": data.Metadata.Namespace,
			},
			"spec": map[string]interface{}{
				"dnsNames": data.Spec.DnsNames,
				"issuerRef": map[string]interface{}{
					"group": data.Spec.IssuerRef.Group,
					"kind":  data.Spec.IssuerRef.Kind,
					"name":  data.Spec.IssuerRef.Name,
				},
				"secretName": data.Spec.SecretName,
			},
		},
	}
}

func loadCertificate(obj *unstructured.Unstructured) (*certificateModel, error) {
	dnsNames, found, err := unstructured.NestedStringSlice(obj.UnstructuredContent(), "spec", "dnsNames")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("field spec.dnsNames not found")
	}

	secretName, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "secretName")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("field spec.secretName not found")
	}

	issuerGroup, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "issuerRef", "group")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("field spec.issuerRef.group not found")
	}

	issuerKind, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "issuerRef", "kind")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("field spec.issuerRef.kind not found")
	}

	issuerName, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "issuerRef", "name")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("field spec.issuerRef.name not found")
	}

	name, found, err := unstructured.NestedString(obj.UnstructuredContent(), "metadata", "name")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no field found for metadata.name")
	}

	namespace, found, err := unstructured.NestedString(obj.UnstructuredContent(), "metadata", "namespace")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no field found for metadata.namespace")
	}

	return &certificateModel{
		Metadata: &certificateMetadata{Name: name, Namespace: namespace},
		Spec: &certificateSpec{
			DnsNames:   dnsNames,
			IssuerRef:  issuerRef{Group: issuerGroup, Kind: issuerKind, Name: issuerName},
			SecretName: secretName,
		},
	}, nil
}
