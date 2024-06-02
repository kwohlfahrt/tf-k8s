package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	strcase "github.com/stoewer/go-strcase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ObjectToState(ctx context.Context, state tfsdk.State, obj *unstructured.Unstructured) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	value, objectDiags := objectToValue(obj.UnstructuredContent(), state.Raw.Type(), path.Empty())
	diags.Append(objectDiags...)
	if objectDiags.HasError() {
		return nil, diags
	}

	// TODO: This seems a bit hacky, can we construct an attr.Type directly?
	stateAttr, err := state.Schema.Type().ValueFromTerraform(ctx, *value)
	if err != nil {
		diags.AddError("Error converting to state type", err.Error())
	}
	return stateAttr, diags
}

func objectToValue(obj interface{}, typ tftypes.Type, path path.Path) (*tftypes.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch typ := typ.(type) {
	case tftypes.Object:
		mapObj, ok := obj.(map[string]interface{})
		if !ok {
			diags.AddAttributeError(path, "Expected object field", fmt.Sprintf("got %T instead", obj))
			break
		}
		tfObj := make(map[string]tftypes.Value, len(mapObj))
		for name, fieldType := range typ.AttributeTypes {
			fieldName := strcase.LowerCamelCase(name)
			fieldObj, ok := mapObj[fieldName]
			if !ok {
				tfObj[name] = tftypes.NewValue(fieldType, nil)
				continue
			}
			fieldValue, fieldDiags := objectToValue(fieldObj, fieldType, path.AtName(name))
			diags.Append(fieldDiags...)
			if fieldDiags.HasError() {
				tfObj[name] = tftypes.NewValue(fieldType, nil)
				continue
			}
			tfObj[name] = *fieldValue
		}
		obj = tfObj
	case tftypes.Map:
		mapObj, ok := obj.(map[string]interface{})
		if !ok {
			diags.AddAttributeError(path, "Expected map field", fmt.Sprintf("got %T instead", obj))
			break
		}
		tfObj := make(map[string]tftypes.Value, len(mapObj))
		for name, fieldObj := range mapObj {
			fieldValue, fieldDiags := objectToValue(fieldObj, typ.ElementType, path.AtMapKey(name))
			diags.Append(fieldDiags...)
			if fieldDiags.HasError() {
				continue
			}
			tfObj[name] = *fieldValue
		}
		obj = tfObj
	case tftypes.List:
		sliceObj, ok := obj.([]interface{})
		if !ok {
			diags.AddAttributeError(path, "Expected list field", fmt.Sprintf("got %T instead", obj))
			break
		}
		tfSlice := make([]tftypes.Value, 0, len(sliceObj))
		for i, value := range sliceObj {
			itemValue, itemDiags := objectToValue(value, typ.ElementType, path.AtListIndex(i))
			diags.Append(itemDiags...)
			if itemDiags.HasError() {
				continue
			}
			tfSlice = append(tfSlice, *itemValue)
		}
		obj = tfSlice
	}

	if err := tftypes.ValidateValue(typ, obj); err != nil {
		diags.AddAttributeError(path, "Invalid value for type", err.Error())
		return nil, diags
	}

	tfValue := tftypes.NewValue(typ, obj)
	return &tfValue, diags
}

func nestedObjectToObject(attr schema.NestedAttributeObject) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes:    attr.Attributes,
		CustomType:    attr.CustomType,
		Validators:    attr.Validators,
		PlanModifiers: attr.PlanModifiers,
	}
}

func objectToNestedObject(attr schema.SingleNestedAttribute) schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes:    attr.Attributes,
		CustomType:    attr.CustomType,
		Validators:    attr.Validators,
		PlanModifiers: attr.PlanModifiers,
	}
}
