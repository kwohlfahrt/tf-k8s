package types

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

func primitiveFromUnstructured(ctx context.Context, path path.Path, typ attr.Type, val interface{}) (attr.Value, diag.Diagnostics) {
	switch typ := typ.(type) {
	case basetypes.StringTypable:
		stringVal, ok := val.(string)
		if !ok {
			return nil, []diag.Diagnostic{diag.NewAttributeErrorDiagnostic(
				path, "Unexpected value", fmt.Sprintf("Expected string, got %T", val),
			)}
		}
		return typ.ValueFromString(ctx, basetypes.NewStringValue(stringVal))
	case basetypes.Int64Typable:
		intVal, ok := val.(int64)
		if !ok {
			return nil, []diag.Diagnostic{diag.NewAttributeErrorDiagnostic(
				path, "Unexpected value", fmt.Sprintf("Expected int64, got %T", val),
			)}
		}
		return typ.ValueFromInt64(ctx, basetypes.NewInt64Value(intVal))
	case basetypes.BoolTypable:
		boolVal, ok := val.(bool)
		if !ok {
			return nil, []diag.Diagnostic{diag.NewAttributeErrorDiagnostic(
				path, "Unexpected value", fmt.Sprintf("Expected bool, got %T", val),
			)}
		}
		return typ.ValueFromBool(ctx, basetypes.NewBoolValue(boolVal))
	default:
		return nil, diag.Diagnostics{diag.NewAttributeErrorDiagnostic(
			path, "Unimplemented value type",
			fmt.Sprintf("Conversion to Kubernetes value is not implemented for %T", val),
		)}
	}
}

func newNull(ctx context.Context, typ attr.Type) attr.Value {
	// AFAIK, this can never throw an error when called this way
	val, _ := typ.ValueFromTerraform(ctx, tftypes.NewValue(typ.TerraformType(ctx), nil))
	return val
}
