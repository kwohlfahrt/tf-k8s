package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func StateToObject(ctx context.Context, state tfsdk.Plan, typeInfo TypeInfo) (*unstructured.Unstructured, diag.Diagnostics) {
	var diags diag.Diagnostics

	obj := make(map[string]interface{}, 4)

	obj["kind"] = typeInfo.Kind
	obj["apiVersion"] = typeInfo.GroupVersionResource().GroupVersion().String()

	var stateValue basetypes.ObjectValue
	diags.Append(state.Get(ctx, &stateValue)...)
	if diags.HasError() {
		return nil, diags
	}
	for k, attr := range stateValue.Attributes() {
		path := path.Root(k)
		if attr.IsNull() || attr.IsUnknown() {
			continue
		}
		objAttr, ok := attr.(types.KubernetesObjectValue)
		if !ok {
			diags.AddAttributeError(
				path, "Unexpected value type",
				fmt.Sprintf("expected object value, got %T", attr))
		}
		attrObj, attrDiags := objAttr.ToUnstructured(ctx, path)
		diags.Append(attrDiags...)
		if attrDiags.HasError() {
			continue
		}
		obj[k] = attrObj
	}

	// Returned object may be inconsistent, if `diags` contains an error
	return &unstructured.Unstructured{Object: obj}, diags
}
