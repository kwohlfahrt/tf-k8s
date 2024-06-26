package types_test

import (
	"context"
	_ "embed"
	"slices"

	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider/crd"
)

func TestRequiredFields(t *testing.T) {
	infoIdx := slices.IndexFunc(crd.TypeInfos, func(typeInfo generic.TypeInfo) bool {
		return typeInfo.Group == "example.com" && typeInfo.Resource == "foos" && typeInfo.Version == "v1"
	})
	if infoIdx == -1 {
		t.Fatal("CRD version not found: foos.example.com/v1")
	}
	typeInfo := crd.TypeInfos[infoIdx]

	result, err := generic.OpenApiToTfSchema(context.Background(), typeInfo.Schema)
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
	infoIdx := slices.IndexFunc(crd.TypeInfos, func(typeInfo generic.TypeInfo) bool {
		return typeInfo.Group == "example.com" && typeInfo.Resource == "foos" && typeInfo.Version == "v1"
	})
	if infoIdx == -1 {
		t.Fatal("CRD version not found: foos.example.com/v1")
	}
	typeInfo := crd.TypeInfos[infoIdx]

	result, err := generic.OpenApiToTfSchema(context.Background(), typeInfo.Schema)
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
