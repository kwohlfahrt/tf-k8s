package types_test

import (
	"context"
	_ "embed"

	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
)

//go:embed test.crds.yaml
var schemaBytes []byte

func TestRequiredFields(t *testing.T) {
	crd, err := generic.LoadCrd(schemaBytes, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if crd == nil {
		t.Fatal("CRD version not found: v1")
	}

	result, err := generic.OpenApiToTfSchema(context.Background(), crd, false)
	if err != nil {
		t.Fatal(err)
	}
	spec := result.Attributes["spec"].(schema.SingleNestedAttribute)
	if !spec.Attributes["foo"].IsRequired() {
		t.Error("Expected attribute spec.foo to be required")
	}
	if spec.Attributes["bar"].IsRequired() {
		t.Error("Expected attribute spec.bar to not be required")
	}
}

func TestFieldType(t *testing.T) {
	crd, err := generic.LoadCrd(schemaBytes, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if crd == nil {
		t.Fatal("CRD version not found: v1")
	}

	result, err := generic.OpenApiToTfSchema(context.Background(), crd, false)
	if err != nil {
		t.Fatal(err)
	}
	spec := result.Attributes["spec"].(schema.SingleNestedAttribute)
	if _, ok := spec.Attributes["foo"].(schema.StringAttribute); !ok {
		t.Error("Expected attribute spec.foo to be string attribute")
	}
	if _, ok := spec.Attributes["bar"].(schema.StringAttribute); !ok {
		t.Error("Expected attribute spec.bar to be string attribute")
	}
}
