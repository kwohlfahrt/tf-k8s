package generic

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

func StateToObject(ctx context.Context, state tfsdk.Plan) (*unstructured.Unstructured, diag.Diagnostics) {
	var diags diag.Diagnostics

	var rawState map[string]tftypes.Value
	if err := state.Raw.As(&rawState); err != nil {
		diags.AddError("Expected object type", fmt.Sprintf("got %s instead", state.Raw.Type().String()))
	}

	obj := make(map[string]interface{}, 4)

	// TODO: Don't hard-code this
	obj["kind"] = "Certificate"
	obj["apiVersion"] = "cert-manager.io/v1"
	rawMetadata := rawState["metadata"]
	metadataObj, metadataDiags := valueToObject(rawMetadata, rawMetadata.Type(), path.Root("metadata"))
	diags.Append(metadataDiags...)
	obj["metadata"] = metadataObj

	var specObj KubernetesObjectValue
	diags.Append(state.GetAttribute(ctx, path.Root("spec"), &specObj)...)
	spec, specDiags := specObj.ToUnstructured(ctx, path.Root("spec"))
	diags.Append(specDiags...)
	if !specDiags.HasError() {
		obj["spec"] = spec
	}

	// Returned object may be inconsistent, if `diags` contains an error
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
			if tfField.IsNull() {
				continue
			}

			field, fieldDiags := valueToObject(tfField, typ.AttributeTypes[name], path.AtName(name))
			diags.Append(fieldDiags...)
			if fieldDiags.HasError() {
				continue
			}
			obj[strcase.LowerCamelCase(name)] = field
		}
		return obj, diags
	case tftypes.Map:
		var tfMap *map[string]tftypes.Value
		if err := value.As(&tfMap); err != nil {
			diags.AddAttributeError(path, "Failed to convert to map", err.Error())
			return nil, diags
		}
		if tfMap == nil {
			return nil, diags
		}
		obj := make(map[string]interface{}, len(*tfMap))
		for name, tfItem := range *tfMap {
			if tfItem.IsNull() {
				continue
			}

			item, itemDiags := valueToObject(tfItem, typ.ElementType, path.AtMapKey(name))
			diags.Append(itemDiags...)
			if itemDiags.HasError() {
				continue
			}
			obj[name] = item
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
