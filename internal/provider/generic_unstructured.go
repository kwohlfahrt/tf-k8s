package provider

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stoewer/go-strcase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func stateToObject(ctx context.Context, state tfsdk.Plan) (*unstructured.Unstructured, diag.Diagnostics) {
	var diags diag.Diagnostics

	var rawState map[string]tftypes.Value
	if err := state.Raw.As(&rawState); err != nil {
		diags.AddError("Expected object type", fmt.Sprintf("got %s instead", state.Raw.Type().String()))
	}

	obj := make(map[string]interface{}, len(rawState)+2)

	// TODO: Don't hard-code this
	obj["kind"] = "Certificate"
	obj["apiVersion"] = "cert-manager.io/v1"
	for name, value := range rawState {
		path := path.Root(name)
		fieldObj, valueDiags := valueToObject(value, value.Type(), path)
		diags.Append(valueDiags...)
		if valueDiags.HasError() {
			continue
		}
		obj[strcase.LowerCamelCase(name)] = fieldObj
	}

	return &unstructured.Unstructured{Object: obj}, diags
}

func valueToObject(value tftypes.Value, typ tftypes.Type, path path.Path) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Complex types
	switch typ := typ.(type) {
	case tftypes.Object:
		var tfObj *map[string]tftypes.Value
		if err := value.As(&tfObj); err != nil {
			diags.AddAttributeError(path, "Failed to convert to object", err.Error())
			return nil, diags
		}
		if tfObj == nil {
			return nil, diags
		}

		obj := make(map[string]interface{}, len(*tfObj))
		for name, tfField := range *tfObj {
			field, fieldDiags := valueToObject(tfField, typ.AttributeTypes[name], path.AtName(name))
			diags.Append(fieldDiags...)
			if fieldDiags.HasError() {
				continue
			}
			obj[strcase.LowerCamelCase(name)] = field
		}
		return obj, diags
	case tftypes.List:
		var tfSlice *[]tftypes.Value
		if err := value.As(&tfSlice); err != nil {
			diags.AddAttributeError(path, "Failed to convert to list", err.Error())
			return nil, diags
		}
		if tfSlice == nil {
			return nil, diags
		}

		obj := make([]interface{}, 0, len(*tfSlice))
		for i, value := range *tfSlice {
			item, itemDiags := valueToObject(value, typ.ElementType, path.AtListIndex(i))
			diags.Append(itemDiags...)
			if itemDiags.HasError() {
				continue
			}
			obj = append(obj, item)
		}
		return obj, diags
	}

	// Primitive Types
	switch {
	case typ.Is(tftypes.Bool):
		var obj *bool
		if err := value.As(&obj); err != nil {
			diags.AddAttributeError(path, "Failed to convert to bool", err.Error())
			return nil, diags
		}
		return *obj, diags
	case typ.Is(tftypes.Number):
		var bigObj *big.Float
		if err := value.As(&bigObj); err != nil {
			diags.AddAttributeError(path, "Failed to convert to float", err.Error())
			return nil, diags
		}
		obj, acc := bigObj.Float64()
		if acc != big.Exact {
			diags.AddAttributeWarning(path, "Float out of range: %s", bigObj.String())
		}
		return obj, diags
	case typ.Is(tftypes.String):
		var obj *string
		if err := value.As(&obj); err != nil {
			diags.AddAttributeError(path, "Failed to convert to string", err.Error())
			return nil, diags
		}
		return *obj, diags
	}

	diags.AddAttributeError(path, "Unsupported field type", fmt.Sprintf("got %T", value))
	return nil, diags
}
