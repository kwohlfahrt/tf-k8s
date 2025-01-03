package types

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func primitiveSchemaType(_ context.Context, attr attr.Type, required bool) (schema.Attribute, error) {
	var schemaType schema.Attribute

	switch attr := attr.(type) {
	case basetypes.StringType:
		schemaType = schema.StringAttribute{
			Required: required,
			Optional: !required,
			Computed: false,
		}
	case basetypes.NumberType:
		schemaType = schema.NumberAttribute{
			Required: required,
			Optional: !required,
			Computed: false,
		}
	case basetypes.Int64Type:
		schemaType = schema.Int64Attribute{
			Required: required,
			Optional: !required,
			Computed: false,
		}
	case basetypes.BoolType:
		schemaType = schema.BoolAttribute{
			Required: required,
			Optional: !required,
			Computed: false,
		}
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
	case basetypes.NumberValuable:
		numberVal, diags := val.ToNumberValue(ctx)
		float, _ := numberVal.ValueBigFloat().Float64()
		return float, diags
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
		if val == nil {
			// Handle case of missing empty strings: github.com/kubernetes/kubernetes#128924.
			// This doesn't solve the conflict when applying though.
			val = ""
		}
		stringVal, ok := val.(string)
		if !ok {
			return nil, []diag.Diagnostic{diag.NewAttributeErrorDiagnostic(
				path, "Unexpected value", fmt.Sprintf("Expected string, got %T", val),
			)}
		}
		return typ.ValueFromString(ctx, basetypes.NewStringValue(stringVal))
	case basetypes.Int64Typable:
		if val == nil {
			// Applies to at least apps/v1/Deployment.spec.minReadySeconds
			// TODO: See if this applies to other fields, or if there is a more generic way to handle it
			return typ.ValueFromInt64(ctx, basetypes.NewInt64Value(0))
		}
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
	case basetypes.NumberTypable:
		floatVal, ok := val.(float64)
		if !ok {
			return nil, []diag.Diagnostic{diag.NewAttributeErrorDiagnostic(
				path, "Unexpected value", fmt.Sprintf("Expected float64, got %T", val),
			)}
		}
		return typ.ValueFromNumber(ctx, basetypes.NewNumberValue(big.NewFloat(floatVal)))
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
