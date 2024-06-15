package types_test

import (
	"context"
	_ "embed"

	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
)

func TestRequiredFields(t *testing.T) {
	typeInfos, err := generic.LoadCrd(internal.SchemaBytes)
	if err != nil {
		t.Fatal(err)
	}
	typeInfo, found := typeInfos["v1"]
	if !found {
		t.Fatal("CRD version not found: v1")
	}

	result, err := generic.OpenApiToTfSchema(context.Background(), typeInfo.Schema, false)
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
	typeInfos, err := generic.LoadCrd(internal.SchemaBytes)
	if err != nil {
		t.Fatal(err)
	}
	typeInfo, found := typeInfos["v1"]
	if !found {
		t.Fatal("CRD version not found: v1")
	}

	result, err := generic.OpenApiToTfSchema(context.Background(), typeInfo.Schema, false)
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
