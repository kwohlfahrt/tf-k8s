package generic

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

func UnstructuredToValue(
	ctx context.Context,
	typ types.KubernetesObjectType,
	obj unstructured.Unstructured,
	fields *fieldpath.Set,
	state *types.KubernetesObjectValue,
) diag.Diagnostics {
	var diags diag.Diagnostics

	value, valueDiags := typ.ValueFromUnstructured(ctx, path.Empty(), fields, obj.UnstructuredContent())
	diags.Append(valueDiags...)
	if valueDiags.HasError() {
		return diags
	}
	*state = value.(types.KubernetesObjectValue)
	return diags
}

func ValueToUnstructured(ctx context.Context, objectValue types.KubernetesValue, typeInfo TypeInfo) (*unstructured.Unstructured, diag.Diagnostics) {
	var diags diag.Diagnostics

	obj, objDiags := objectValue.ToUnstructured(ctx, path.Empty())
	diags.Append(objDiags...)
	if diags.HasError() {
		return nil, diags
	}

	objMap := obj.(map[string]interface{})
	objMap["kind"] = typeInfo.Kind
	objMap["apiVersion"] = typeInfo.GroupVersionResource().GroupVersion().String()

	return &unstructured.Unstructured{Object: objMap}, diags
}
