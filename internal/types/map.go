package types

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type KubernetesMapType struct {
	basetypes.DynamicType

	ElemType attr.Type
}

func (t KubernetesMapType) Equal(o attr.Type) bool {
	other, ok := o.(KubernetesMapType)
	if !ok {
		return false
	}

	return t.DynamicType.Equal(other.DynamicType)
}

func (t KubernetesMapType) String() string {
	return "KubernetesMapType"
}

func (t KubernetesMapType) ValueFromDynamic(ctx context.Context, in basetypes.DynamicValue) (basetypes.DynamicValuable, diag.Diagnostics) {
	var diags diag.Diagnostics
	value := KubernetesMapValue{DynamicValue: in, elemType: t.ElemType}
	if in.IsNull() || in.IsUnderlyingValueNull() || in.IsUnknown() || in.IsUnderlyingValueUnknown() {
		return value, diags
	}

	underlying := in.UnderlyingValue()
	switch underlying.(type) {
	case basetypes.MapValue, basetypes.ObjectValue:
		return value, diags
	default:
		diags.Append(diag.NewErrorDiagnostic("Unexpected value type", fmt.Sprintf("Expected MapValue, got %T", underlying)))
		return nil, diags
	}
}

func (t KubernetesMapType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	var obj basetypes.ObjectValue
	switch {
	case in.IsNull():
		obj = basetypes.NewObjectNull(map[string]attr.Type{})
	case !in.IsKnown():
		obj = basetypes.NewObjectUnknown(map[string]attr.Type{})
	default:
		inObj := make(map[string]tftypes.Value, 0)
		if err := in.As(&inObj); err != nil {
			return nil, err
		}
		elems := make(map[string]attr.Value, len(inObj))
		elemTypes := make(map[string]attr.Type, len(inObj))
		for k, v := range inObj {
			elem, err := t.ElemType.ValueFromTerraform(ctx, v)
			if err != nil {
				return nil, err
			}
			elems[k] = elem
			elemTypes[k] = t.ElemType
		}
		obj = basetypes.NewObjectValueMust(elemTypes, elems)
	}

	kubernetesValue, _ := t.ValueFromDynamic(ctx, basetypes.NewDynamicValue(obj))
	return kubernetesValue, nil
}

func (t KubernetesMapType) ValueType(ctx context.Context) attr.Value {
	return KubernetesMapValue{elemType: t.ElemType}
}

func (t KubernetesMapType) ValueFromUnstructured(
	ctx context.Context,
	path path.Path,
	fields *fieldpath.Set,
	obj interface{},
) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj == nil {
		obj = make(map[string]interface{}, 0)
	}

	mapObj, ok := obj.(map[string]interface{})
	if !ok {
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type",
			fmt.Sprintf("Expected map of items, got %T", obj),
		))
		return nil, diags
	}

	elems := make(map[string]attr.Value, len(mapObj))
	elemTypes := make(map[string]attr.Type, len(mapObj))
	for k, value := range mapObj {
		elemPath := path.AtMapKey(k)

		var elem attr.Value
		var attrDiags diag.Diagnostics
		p := fieldpath.PathElement{FieldName: &k}
		if kubernetesElemType, ok := t.ElemType.(KubernetesType); ok {
			if fields == nil || fields.Members.Has(p) {
				elem, attrDiags = kubernetesElemType.ValueFromUnstructured(ctx, elemPath, nil, value)
			} else if childFields, found := fields.Children.Get(p); found {
				elem, attrDiags = kubernetesElemType.ValueFromUnstructured(ctx, elemPath, childFields, value)
			} else {
				continue
			}
		} else {
			if fields == nil || fields.Members.Has(p) {
				elem, attrDiags = primitiveFromUnstructured(ctx, elemPath, t.ElemType, value)
			} else {
				continue
			}
		}
		diags.Append(attrDiags...)
		if attrDiags.HasError() {
			continue
		}
		elems[k] = elem
		elemTypes[k] = t.ElemType
	}

	baseMap, mapDiags := basetypes.NewObjectValue(elemTypes, elems)
	diags.Append(mapDiags...)
	result, mapDiags := t.ValueFromDynamic(ctx, basetypes.NewDynamicValue(baseMap))
	diags.Append(mapDiags...)

	return result, diags
}

func (t KubernetesMapType) SchemaType(ctx context.Context, opts SchemaOptions, isRequired bool) (schema.Attribute, error) {
	return schema.DynamicAttribute{
		Required:   isRequired,
		Optional:   !isRequired,
		Computed:   false,
		CustomType: t,
	}, nil
}

func MapFromOpenApi(root *spec3.OpenAPI, openapi spec.Schema, path []string) (KubernetesType, error) {
	items := openapi.AdditionalProperties.Schema
	if items == nil {
		return nil, fmt.Errorf("expected map of items at %s", strings.Join(path, ""))
	}

	elemType, err := OpenApiToTfType(root, *items, append(path, "[*]"))
	if err != nil {
		return nil, err
	}

	return KubernetesMapType{DynamicType: basetypes.DynamicType{}, ElemType: elemType}, nil
}

var _ basetypes.DynamicTypable = KubernetesMapType{}
var _ KubernetesType = KubernetesMapType{}

type KubernetesMapValue struct {
	basetypes.DynamicValue

	elemType attr.Type
}

func (v KubernetesMapValue) Attributes() map[string]attr.Value {
	return v.UnderlyingValue().(basetypes.ObjectValue).Attributes()
}

func (v KubernetesMapValue) ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	elems := v.Attributes()
	result := make(map[string]interface{}, len(elems))
	for k, elem := range elems {
		elemPath := path.AtMapKey(k)
		var elemObj interface{}
		var elemDiags diag.Diagnostics
		if kubernetesAttr, ok := elem.(KubernetesValue); ok {
			elemObj, elemDiags = kubernetesAttr.ToUnstructured(ctx, elemPath)
		} else {
			elemObj, elemDiags = primitiveToUnstructured(ctx, elemPath, elem)
		}
		diags.Append(elemDiags...)
		if elemDiags.HasError() {
			continue
		}
		result[k] = elemObj
	}
	return result, nil
}

func (v KubernetesMapValue) Equal(o attr.Value) bool {
	other, ok := o.(KubernetesMapValue)
	if !ok {
		return false
	}
	return v.DynamicValue.Equal(other.DynamicValue)
}

func (v KubernetesMapValue) Type(ctx context.Context) attr.Type {
	return KubernetesMapType{DynamicType: basetypes.DynamicType{}, ElemType: v.elemType}
}

func (v KubernetesMapValue) ManagedFields(ctx context.Context, path path.Path, fields *fieldpath.Set, pe *fieldpath.PathElement) diag.Diagnostics {
	var diags diag.Diagnostics

	fields = fields.Children.Descend(*pe)
	elems := v.Attributes()
	for k, elem := range elems {
		if elem.IsNull() {
			continue
		}

		fieldPath := path.AtMapKey(k)
		pathElem := fieldpath.PathElement{FieldName: &k}
		if kubernetesAttr, ok := elem.(KubernetesValue); ok {
			diags.Append(kubernetesAttr.ManagedFields(ctx, fieldPath, fields, &pathElem)...)
		} else {
			fields.Insert([]fieldpath.PathElement{pathElem})
		}
	}

	return diags
}

func (v KubernetesMapValue) Validate(ctx context.Context, path path.Path) diag.Diagnostics {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.IsUnderlyingValueNull() || v.IsUnderlyingValueUnknown() {
		return diags
	}

	for k, elem := range v.Attributes() {
		if kubernetesElem, ok := elem.(KubernetesValue); ok {
			diags.Append(kubernetesElem.Validate(ctx, path.AtMapKey(k))...)
		}
	}

	return diags
}

var _ basetypes.DynamicValuable = KubernetesMapValue{}
var _ KubernetesValue = KubernetesMapValue{}
