package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ObjectToState(ctx context.Context, typ types.KubernetesObjectType, obj unstructured.Unstructured) (types.KubernetesValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	value, valueDiags := typ.ValueFromUnstructured(ctx, path.Empty(), obj.UnstructuredContent())
	diags.Append(valueDiags...)
	if valueDiags.HasError() {
		return nil, diags
	}
	kubernetesValue, ok := value.(*types.KubernetesObjectValue)
	if !ok {
		diags.AddError(
			"Unexpected value type",
			fmt.Sprintf("Expected KubernetesObjectValue, got %T. This is a provider-internal error.", value),
		)
	}

	return kubernetesValue, diags
}

type PlanOrState interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

func FillNulls(ctx context.Context, value types.KubernetesValue, state PlanOrState) diag.Diagnostics {
	var diags diag.Diagnostics

	var stateValue basetypes.ObjectValue
	diags.Append(state.Get(ctx, &stateValue)...)
	if diags.HasError() {
		return diags
	}

	diags.Append(value.FillNulls(ctx, path.Empty(), stateValue)...)
	return diags
}
