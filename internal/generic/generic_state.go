package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ObjectToState(ctx context.Context, state tfsdk.State, obj *unstructured.Unstructured) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	content := obj.UnstructuredContent()
	attributes := make(map[string]attr.Value, 3)

	for _, k := range []string{"metadata", "spec"} {
		path := path.Root(k)
		fieldType, fieldDiags := state.Schema.TypeAtPath(ctx, path)
		diags.Append(fieldDiags...)
		if fieldDiags.HasError() {
			continue
		}

		kubernetesType, ok := fieldType.(KubernetesType)
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
		// TODO: Better error handling here.
		metadata := content["metadata"].(map[string]interface{})
		name := metadata["name"].(string)
		namespace := metadata["namespace"].(string)
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
