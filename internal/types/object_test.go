package types_test

import (
	"context"
	_ "embed"

	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
)

var testSchema types.KubernetesObjectType = types.KubernetesObjectType{
	ObjectType: basetypes.ObjectType{
		AttrTypes: map[string]attr.Type{
			"metadata": types.KubernetesObjectType{
				ObjectType: basetypes.ObjectType{
					AttrTypes: map[string]attr.Type{
						"name":      basetypes.StringType{},
						"namespace": basetypes.StringType{},
					},
				},
				FieldNames:     map[string]string{"name": "name", "namespace": "namespace"},
				RequiredFields: map[string]bool{"name": true, "namespace": true},
			},
			"spec": types.KubernetesObjectType{
				ObjectType: basetypes.ObjectType{
					AttrTypes: map[string]attr.Type{
						"foo": basetypes.StringType{},
						"bar": basetypes.StringType{},
					},
				},
				FieldNames:     map[string]string{"foo": "foo", "bar": "bar"},
				RequiredFields: map[string]bool{"foo": true},
			},
		},
	},
	FieldNames: map[string]string{"spec": "spec", "metadata": "metadata"},
}

func TestRequiredFields(t *testing.T) {
	result, err := generic.OpenApiToTfSchema(context.Background(), testSchema, false)
	if err != nil {
		t.Fatal(err)
	}
	spec := result.(schema.SingleNestedAttribute).Attributes["spec"].(schema.SingleNestedAttribute)
	if !spec.Attributes["foo"].IsRequired() {
		t.Error("Expected attribute spec.foo to be required")
	}
	if spec.Attributes["bar"].IsRequired() {
		t.Error("Expected attribute spec.bar to not be required")
	}
}

func TestFieldType(t *testing.T) {
	result, err := generic.OpenApiToTfSchema(context.Background(), testSchema, false)
	if err != nil {
		t.Fatal(err)
	}
	spec := result.(schema.SingleNestedAttribute).Attributes["spec"].(schema.SingleNestedAttribute)
	if _, ok := spec.Attributes["foo"].(schema.StringAttribute); !ok {
		t.Error("Expected attribute spec.foo to be string attribute")
	}
	if _, ok := spec.Attributes["bar"].(schema.StringAttribute); !ok {
		t.Error("Expected attribute spec.bar to be string attribute")
	}
}
