package generic

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func StateToObject(ctx context.Context, state tfsdk.Plan, typeInfo TypeInfo) (*unstructured.Unstructured, diag.Diagnostics) {
	var diags diag.Diagnostics

	obj := make(map[string]interface{}, 4)

	obj["kind"] = typeInfo.Kind
	obj["apiVersion"] = typeInfo.GroupVersionResource().GroupVersion().String()

	var metaObj types.KubernetesObjectValue
	diags.Append(state.GetAttribute(ctx, path.Root("metadata"), &metaObj)...)
	meta, metaDiags := metaObj.ToUnstructured(ctx, path.Root("metadata"))
	diags.Append(metaDiags...)
	if !metaDiags.HasError() {
		obj["metadata"] = meta
	}

	var specObj types.KubernetesObjectValue
	diags.Append(state.GetAttribute(ctx, path.Root("spec"), &specObj)...)
	spec, specDiags := specObj.ToUnstructured(ctx, path.Root("spec"))
	diags.Append(specDiags...)
	if !specDiags.HasError() {
		obj["spec"] = spec
	}

	// Returned object may be inconsistent, if `diags` contains an error
	return &unstructured.Unstructured{Object: obj}, diags
}
