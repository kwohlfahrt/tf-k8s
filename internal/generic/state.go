package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ObjectToState(ctx context.Context, state tfsdk.State, obj *unstructured.Unstructured) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	content := obj.UnstructuredContent()
	schemaAttributes := state.Schema.GetAttributes()
	attributes := make(map[string]attr.Value, len(schemaAttributes))

	for k, attr := range schemaAttributes {
		path := path.Root(k)
		if k == "id" {
			continue
		}

		fieldType := attr.GetType()
		kubernetesType, ok := fieldType.(types.KubernetesType)
		if !ok {
			diags.AddAttributeError(path, "Unexpected schema type", fmt.Sprintf("Expected KubernetesType, got %T", fieldType))
			continue
		}
		val, fieldDiags := kubernetesType.ValueFromUnstructured(ctx, path, content[k])
		diags.Append(fieldDiags...)
		if fieldDiags.HasError() {
			continue
		}

		attributes[k] = val
	}

	if _, needsId := state.Schema.GetAttributes()["id"]; needsId {
		metadata := content["metadata"].(map[string]interface{})
		name, ok := metadata["name"].(string)
		if !ok {
			diags.Append(diag.NewAttributeErrorDiagnostic(
				path.Root("metadata").AtName("name"),
				"Expected value not found",
				fmt.Sprintf("Expected string, got %T", metadata["name"]),
			))
		}
		namespace, ok := metadata["namespace"].(string)
		if !ok {
			diags.Append(diag.NewAttributeErrorDiagnostic(
				path.Root("metadata").AtName("namespace"),
				"Expected value not found",
				fmt.Sprintf("Expected string, got %T", metadata["namespace"]),
			))
		}
		id := basetypes.NewStringValue(fmt.Sprintf("%s/%s", namespace, name))
		attributes["id"] = id
	}

	typ, ok := state.Schema.Type().(basetypes.ObjectType)
	if !ok {
		diags.AddError("Unexpected schema type", fmt.Sprintf("Expected ObjectType, got %T", typ))
		return nil, diags
	}
	value, stateDiags := basetypes.NewObjectValue(typ.AttrTypes, attributes)
	diags.Append(stateDiags...)

	return value, diags
}
