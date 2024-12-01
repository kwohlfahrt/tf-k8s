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
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type KubernetesUnknownType struct {
	basetypes.DynamicType
}

func (t KubernetesUnknownType) Equal(o attr.Type) bool {
	other, ok := o.(KubernetesUnknownType)
	if !ok {
		return false
	}

	return t.DynamicType.Equal(other.DynamicType)
}

func (t KubernetesUnknownType) SchemaType(ctx context.Context, opts SchemaOptions, isRequired bool) (schema.Attribute, error) {
	return schema.DynamicAttribute{
		CustomType: t,
		Required:   isRequired,
		Optional:   !isRequired,
		Computed:   false,
	}, nil
}

func (t KubernetesUnknownType) String() string {
	return "KubernetesUnknownType"
}

func (t KubernetesUnknownType) ValueFromDynamic(ctx context.Context, in basetypes.DynamicValue) (basetypes.DynamicValuable, diag.Diagnostics) {
	value := KubernetesUnknownValue{DynamicValue: in}
	return &value, nil
}

func (t KubernetesUnknownType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	value, err := t.DynamicType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	dynamicValue, ok := value.(basetypes.DynamicValue)
	if !ok {
		return nil, fmt.Errorf("expected DynamicValue, got %T", value)
	}

	dynamicValuable, diags := t.ValueFromDynamic(ctx, dynamicValue)
	if diags.HasError() {
		return nil, fmt.Errorf("error converting DynamicValue to DynamicValuable: %v", diags)
	}

	return dynamicValuable, nil
}

func (t KubernetesUnknownType) ValueFromUnstructured(ctx context.Context, path path.Path, fields *fieldpath.Set, obj interface{}) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	var value attr.Value

	if fields != nil {
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected field-set for dynamic value",
			"Extracting managed fields from a dynamic value is not supported",
		))
	}

	switch obj := obj.(type) {
	case map[string]interface{}:
		typs := make(map[string]attr.Type, len(obj))
		attrs := make(map[string]attr.Value, len(obj))
		for k, attrObj := range obj {
			attr, attrDiags := t.ValueFromUnstructured(ctx, path.AtName(k), nil, attrObj)
			diags.Append(attrDiags...)
			if !attrDiags.HasError() {
				attrs[k] = attr
				typs[k] = attr.Type(ctx)
			}
		}
		objValue, objDiags := basetypes.NewObjectValue(typs, attrs)
		diags.Append(objDiags...)
		value = objValue
	case []interface{}:
		typs := make([]attr.Type, 0, len(obj))
		elems := make([]attr.Value, 0, len(obj))
		for i, elemObj := range obj {
			elem, elemDiags := t.ValueFromUnstructured(ctx, path.AtListIndex(i), nil, elemObj)
			diags.Append(elemDiags...)
			if !elemDiags.HasError() {
				elems = append(elems, elem)
				typs = append(typs, elem.Type(ctx))
			}
		}
		tupleValue, tupleDiags := basetypes.NewTupleValue(typs, elems)
		diags.Append(tupleDiags...)
		value = tupleValue
	case int64:
		value = basetypes.NewInt64Value(obj)
	case float64:
		value = basetypes.NewFloat64Value(obj)
	case string:
		value = basetypes.NewStringValue(obj)
	case bool:
		value = basetypes.NewBoolValue(obj)
	default:
		diags.Append(diag.NewAttributeErrorDiagnostic(path, "Unimplemented", fmt.Sprintf("Unknown value support for type %T not implemented", obj)))
		return nil, diags
	}
	dynamicValue := basetypes.NewDynamicValue(value)
	return KubernetesUnknownValue{DynamicValue: dynamicValue}, diags
}

var _ basetypes.DynamicTypable = KubernetesUnknownType{}
var _ KubernetesType = KubernetesUnknownType{}

type KubernetesUnknownValue struct {
	basetypes.DynamicValue
}

func (v KubernetesUnknownValue) Equal(o attr.Value) bool {
	other, ok := o.(KubernetesUnknownValue)
	if !ok {
		return false
	}
	return v.DynamicValue.Equal(other.DynamicValue)
}

func (v KubernetesUnknownValue) Type(ctx context.Context) attr.Type {
	return KubernetesUnknownType{
		DynamicType: basetypes.DynamicType{},
	}
}

func (v KubernetesUnknownValue) ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics) {
	return DynamicToUnstructured(v.UnderlyingValue(), path)
}

func DynamicToUnstructured(obj attr.Value, path path.Path) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}

	switch v := obj.(type) {
	case basetypes.ObjectValue:
		attrs := v.Attributes()
		unstructured := make(map[string]interface{}, len(attrs))
		for k, v := range attrs {
			attrPath := path.AtName(k)
			attr, attrDiags := DynamicToUnstructured(v, attrPath)
			diags.Append(attrDiags...)
			if !attrDiags.HasError() {
				unstructured[k] = attr
			}
		}
		return unstructured, diags
	case basetypes.TupleValue:
		elems := v.Elements()
		unstructured := make([]interface{}, 0, len(elems))
		for i, v := range elems {
			elemPath := path.AtTupleIndex(i)
			elem, elemDiags := DynamicToUnstructured(v, elemPath)
			diags.Append(elemDiags...)
			if !elemDiags.HasError() {
				unstructured = append(unstructured, elem)
			}
		}
		return unstructured, diags
	case basetypes.ListValue:
		elems := v.Elements()
		unstructured := make([]interface{}, 0, len(elems))
		for i, v := range elems {
			elemPath := path.AtListIndex(i)
			elem, elemDiags := DynamicToUnstructured(v, elemPath)
			diags.Append(elemDiags...)
			if !elemDiags.HasError() {
				unstructured = append(unstructured, elem)
			}
		}
		return unstructured, diags
	case basetypes.MapValue:
		elems := v.Elements()
		unstructured := make(map[string]interface{}, len(elems))
		for k, v := range elems {
			attrPath := path.AtMapKey(k)
			attr, attrDiags := DynamicToUnstructured(v, attrPath)
			diags.Append(attrDiags...)
			if !attrDiags.HasError() {
				unstructured[k] = attr
			}
		}
		return unstructured, diags
	case basetypes.SetValue:
		elems := v.Elements()
		unstructured := make([]interface{}, 0, len(elems))
		for _, v := range elems {
			elemPath := path.AtSetValue(v)
			elem, elemDiags := DynamicToUnstructured(v, elemPath)
			diags.Append(elemDiags...)
			if !elemDiags.HasError() {
				unstructured = append(unstructured, elem)
			}
		}
		return unstructured, diags
	case basetypes.StringValue:
		return v.ValueString(), diags
	case basetypes.NumberValue:
		f := v.ValueBigFloat()
		if f.IsInt() {
			i, _ := f.Int64()
			return i, diags
		} else {
			f, _ := f.Float64()
			return f, diags
		}
	case basetypes.BoolValue:
		return v.ValueBool(), diags
	default:
		diags.Append(diag.NewAttributeErrorDiagnostic(path, "Unsupported dynamic value type", fmt.Sprintf("got %T", v)))
		return nil, diags
	}
}

func (v KubernetesUnknownValue) ManagedFields(ctx context.Context, path path.Path, fields *fieldpath.Set, pe *fieldpath.PathElement) diag.Diagnostics {
	value := v.UnderlyingValue()
	if kubernetesValue, ok := value.(KubernetesValue); ok {
		return kubernetesValue.ManagedFields(ctx, path, fields, pe)
	} else {
		fields.Insert([]fieldpath.PathElement{*pe})
		return nil
	}
}

var _ basetypes.DynamicValuable = KubernetesUnknownValue{}
var _ KubernetesValue = KubernetesUnknownValue{}
