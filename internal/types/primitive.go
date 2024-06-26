package types

import (
	"context"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func primitiveSchemaType(_ context.Context, attr attr.Type, required bool) (schema.Attribute, error) {
	var schemaType schema.Attribute

	switch attr := attr.(type) {
	case basetypes.StringType:
		schemaType = schema.StringAttribute{
			Required:      required,
			Optional:      !required,
			Computed:      !required,
			PlanModifiers: []planmodifier.String{makeStringPlanModifier()},
		}
	case basetypes.Int64Type:
		schemaType = schema.Int64Attribute{
			Required:      required,
			Optional:      !required,
			Computed:      !required,
			PlanModifiers: []planmodifier.Int64{makeInt64PlanModifier()},
		}
	case basetypes.BoolType:
		schemaType = schema.BoolAttribute{
			Required:      required,
			Optional:      !required,
			Computed:      !required,
			PlanModifiers: []planmodifier.Bool{makeBoolPlanModifier()},
		}
	default:
		return nil, fmt.Errorf("no schema for type %T", attr)
	}
	return schemaType, nil
}

type stringPlanModifier struct{}

func (o stringPlanModifier) Description(context.Context) string {
	return ""
}

func (o stringPlanModifier) MarkdownDescription(context.Context) string {
	return ""
}

func (o stringPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() {
		// Create phase, keep null values
		return
	}
	if !req.PlanValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsUnknown() {
		return
	}

	resp.PlanValue = req.StateValue
}

func makeStringPlanModifier() planmodifier.String {
	return stringPlanModifier{}
}

type int64PlanModifier struct{}

func (o int64PlanModifier) Description(context.Context) string {
	return ""
}

func (o int64PlanModifier) MarkdownDescription(context.Context) string {
	return ""
}

func (o int64PlanModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if req.State.Raw.IsNull() {
		// Create phase, keep null values
		return
	}
	if !req.PlanValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsUnknown() {
		return
	}

	resp.PlanValue = req.StateValue
}

func makeInt64PlanModifier() planmodifier.Int64 {
	return int64PlanModifier{}
}

type boolPlanModifier struct{}

func (o boolPlanModifier) Description(context.Context) string {
	return ""
}

func (o boolPlanModifier) MarkdownDescription(context.Context) string {
	return ""
}

func (o boolPlanModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.State.Raw.IsNull() {
		// Create phase, keep null values
		return
	}
	if !req.PlanValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsUnknown() {
		return
	}

	resp.PlanValue = req.StateValue
}

func makeBoolPlanModifier() planmodifier.Bool {
	return boolPlanModifier{}
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

func dynamicPrimitiveFromUnstructured(ctx context.Context, path path.Path, val interface{}) (attr.Value, diag.Diagnostics) {
	switch val := val.(type) {
	case string:
		return basetypes.NewStringValue(val), nil
	case int64:
		return basetypes.NewInt64Value(val), nil
	case bool:
		return basetypes.NewBoolValue(val), nil
	}
	return nil, diag.Diagnostics{diag.NewAttributeErrorDiagnostic(
		path, "Unimplemented value type",
		fmt.Sprintf("Conversion to Kubernetes value is not implemented for %T", val),
	)}
}

// Tuples don't have associated schema types, so just treat them as a black-box primitive
func dynamicTupleFromUnstructured(ctx context.Context, path path.Path, val []interface{}) (basetypes.TupleValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemValues := make([]attr.Value, 0, len(val))
	elemTypes := make([]attr.Type, 0, len(val))
	for i, v := range val {
		fieldPath := path.AtTupleIndex(i)
		var elemValue attr.Value
		var elemDiags diag.Diagnostics
		switch v := v.(type) {
		case map[string]interface{}:
			elemValue, elemDiags = DynamicObjectFromUnstructured(ctx, fieldPath, v)
		case []interface{}:
			elemValue, elemDiags = dynamicTupleFromUnstructured(ctx, fieldPath, v)
		default:
			elemValue, elemDiags = dynamicPrimitiveFromUnstructured(ctx, fieldPath, v)
		}
		diags.Append(elemDiags...)
		if elemDiags.HasError() {
			continue
		}

		elemValues = append(elemValues, elemValue)
		elemTypes = append(elemTypes, elemValue.Type(ctx))
	}

	obj, objDiags := basetypes.NewTupleValue(elemTypes, elemValues)
	diags.Append(objDiags...)
	return obj, diags
}

func primitiveCodegen(attr interface{}, builder io.StringWriter) error {
	var err error
	switch attr := attr.(type) {
	case basetypes.StringType:
		_, err = builder.WriteString("basetypes.StringType{}")
	case basetypes.Int64Type:
		_, err = builder.WriteString("basetypes.Int64Type{}")
	case basetypes.BoolType:
		_, err = builder.WriteString("basetypes.BoolType{}")
	default:
		err = fmt.Errorf("no codegen for type %T", attr)
	}
	return err
}

func isPrimitive(openapi spec.Schema) bool {
	if len(openapi.Type) > 0 {
		for _, typ := range openapi.Type {
			switch typ {
			case "integer", "number", "string":
				continue
			default:
				return false
			}
		}
		return true
	}
	if len(openapi.OneOf) > 0 {
		for _, typ := range openapi.OneOf {
			if !isPrimitive(typ) {
				return false
			}
		}
		return true
	}
	return false
}
