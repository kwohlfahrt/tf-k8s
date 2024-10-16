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
)

type KubernetesMapType struct {
	basetypes.MapType
}

func (t KubernetesMapType) Equal(o attr.Type) bool {
	other, ok := o.(KubernetesMapType)
	if !ok {
		return false
	}

	return t.MapType.Equal(other.MapType)
}

func (t KubernetesMapType) String() string {
	return "KubernetesMapType"
}

func (t KubernetesMapType) ValueFromMap(ctx context.Context, in basetypes.MapValue) (basetypes.MapValuable, diag.Diagnostics) {
	value := KubernetesMapValue{MapValue: in}
	return &value, nil
}

func (t KubernetesMapType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.MapType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	mapValue, ok := attrValue.(basetypes.MapValue)
	if !ok {
		return nil, fmt.Errorf("expected MapValue, got %T", attrValue)
	}

	mapValuable, diags := t.ValueFromMap(ctx, mapValue)
	if diags.HasError() {
		return nil, fmt.Errorf("error converting MapValue to MapValuable: %v", diags)
	}

	return mapValuable, nil
}

func (t KubernetesMapType) ValueType(ctx context.Context) attr.Value {
	return KubernetesMapValue{}
}

func (t KubernetesMapType) ValueFromUnstructured(ctx context.Context, path path.Path, obj interface{}) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	mapObj, ok := obj.(map[string]interface{})
	if !ok {
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type",
			fmt.Sprintf("Expected map of items, got %T", obj),
		))
		return nil, diags
	}

	elems := make(map[string]attr.Value, len(mapObj))
	for k, value := range mapObj {
		elemPath := path.AtMapKey(k)

		var elem attr.Value
		var attrDiags diag.Diagnostics
		if kubernetesElem, ok := t.ElemType.(KubernetesType); ok {
			elem, attrDiags = kubernetesElem.ValueFromUnstructured(ctx, elemPath, value)
		} else {
			elem, attrDiags = primitiveFromUnstructured(ctx, elemPath, t.ElemType, value)
		}
		diags.Append(attrDiags...)
		if attrDiags.HasError() {
			continue
		}
		elems[k] = elem
	}

	baseMap, mapDiags := basetypes.NewMapValue(t.ElemType, elems)
	diags.Append(mapDiags...)
	result, mapDiags := t.ValueFromMap(ctx, baseMap)
	diags.Append(mapDiags...)

	return result, diags
}

func (t KubernetesMapType) SchemaType(ctx context.Context, required bool) (schema.Attribute, error) {
	elem := t.ElementType()
	if objectElem, ok := elem.(KubernetesObjectType); ok {
		attributes, err := objectElem.SchemaAttributes(ctx, required)
		if err != nil {
			return nil, err
		}
		return schema.MapNestedAttribute{
			Required:   required,
			Optional:   !required,
			Computed:   false,
			CustomType: t,
			NestedObject: schema.NestedAttributeObject{
				Attributes: attributes,
				CustomType: objectElem,
			},
		}, nil
	} else {
		return schema.MapAttribute{
			Required:    required,
			Optional:    !required,
			Computed:    false,
			CustomType:  t,
			ElementType: elem,
		}, nil
	}
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

	return KubernetesMapType{MapType: basetypes.MapType{ElemType: elemType}}, nil
}

var _ basetypes.MapTypable = KubernetesMapType{}
var _ KubernetesType = KubernetesMapType{}

type KubernetesMapValue struct {
	basetypes.MapValue
}

func (v KubernetesMapValue) ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	elems := v.MapValue.Elements()
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
	return v.MapValue.Equal(other.MapValue)
}

func (v KubernetesMapValue) Type(ctx context.Context) attr.Type {
	return KubernetesMapType{MapType: basetypes.MapType{ElemType: v.ElementType(ctx)}}
}

func (v *KubernetesMapValue) FillNulls(ctx context.Context, path path.Path, config attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics

	var kubernetesConfig *KubernetesMapValue
	switch config := config.(type) {
	case basetypes.MapValue:
		baseConfig, diags := v.Type(ctx).(KubernetesMapType).ValueFromMap(ctx, config)
		if diags.HasError() {
			return diags
		}
		kubernetesConfig = baseConfig.(*KubernetesMapValue)
	case *KubernetesMapValue:
		kubernetesConfig = config
	default:
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type",
			fmt.Sprintf("Expected MapValue, got %T", config),
		))
		return diags
	}

	configElements := kubernetesConfig.Elements()
	if v.IsNull() && !kubernetesConfig.IsNull() && len(configElements) == 0 {
		v.MapValue, diags = basetypes.NewMapValue(v.ElementType(ctx), map[string]attr.Value{})
	} else if !v.IsNull() && kubernetesConfig.IsNull() && len(v.Elements()) == 0 {
		v.MapValue = basetypes.NewMapNull(v.ElementType(ctx))
	} else {
		for k, v := range v.Elements() {
			if kubernetesValue, ok := v.(KubernetesValue); ok {
				kubernetesValue.FillNulls(ctx, path.AtMapKey(k), configElements[k])
			}
		}
	}

	return diags
}

var _ basetypes.MapValuable = KubernetesMapValue{}
var _ KubernetesValue = &KubernetesMapValue{}
