package generic

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func StateToObject(ctx context.Context, state tfsdk.Plan, typeInfo TypeInfo) (*unstructured.Unstructured, diag.Diagnostics) {
	var diags diag.Diagnostics

	var stateValue basetypes.ObjectValue
	diags.Append(state.Get(ctx, &stateValue)...)
	if diags.HasError() {
		return nil, diags
	}

	value, valueDiags := typeInfo.Schema.ValueFromObject(ctx, stateValue)
	diags.Append(valueDiags...)
	if diags.HasError() {
		return nil, diags
	}
	objectValue := value.(types.KubernetesObjectValue)
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
