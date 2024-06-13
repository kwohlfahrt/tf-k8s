package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func primitiveSchemaType(_ context.Context, attr attr.Type, isDatasource, isRequired bool) (schema.Attribute, error) {
	computed := isDatasource
	optional := !isDatasource && !isRequired
	required := !isDatasource && isRequired
	var schemaType schema.Attribute

	switch attr := attr.(type) {
	case basetypes.StringType:
		schemaType = schema.StringAttribute{Required: required, Optional: optional, Computed: computed}
	case basetypes.Int64Type:
		schemaType = schema.Int64Attribute{Required: required, Optional: optional, Computed: computed}
	case basetypes.BoolType:
		schemaType = schema.BoolAttribute{Required: required, Optional: optional, Computed: computed}
	default:
		return nil, fmt.Errorf("no schema for type %T", attr)
	}
	return schemaType, nil
}

func primitiveToUnstructured(ctx context.Context, path path.Path, val attr.Value) (interface{}, diag.Diagnostics) {
	switch val := val.(type) {
	case basetypes.StringValuable:
		stringVal, diags := val.ToStringValue(ctx)
		return stringVal.ValueString(), diags
	case basetypes.Int64Valuable:
		intVal, diags := val.ToInt64Value(ctx)
		return intVal.ValueInt64(), diags
	case basetypes.BoolValuable:
		boolVal, diags := val.ToBoolValue(ctx)
		return boolVal.ValueBool(), diags
	default:
		return nil, diag.Diagnostics{diag.NewAttributeErrorDiagnostic(
			path, "Unimplemented value type",
			fmt.Sprintf("Conversion to Kubernetes value is not implemented for %T", val),
		)}
	}
}
