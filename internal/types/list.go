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

type KubernetesListType struct {
	basetypes.ListType
}

func (t KubernetesListType) Equal(o attr.Type) bool {
	other, ok := o.(KubernetesListType)
	if !ok {
		return false
	}

	return t.ListType.Equal(other.ListType)
}

func (t KubernetesListType) String() string {
	return "KubernetesListType"
}

func (t KubernetesListType) ValueFromList(ctx context.Context, in basetypes.ListValue) (basetypes.ListValuable, diag.Diagnostics) {
	value := KubernetesListValue{ListValue: in}
	return &value, nil
}

func (t KubernetesListType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.ListType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	listValue, ok := attrValue.(basetypes.ListValue)
	if !ok {
		return nil, fmt.Errorf("expected ListValue, got %T", attrValue)
	}

	listValuable, diags := t.ValueFromList(ctx, listValue)
	if diags.HasError() {
		return nil, fmt.Errorf("error converting ListValue to ListValuable: %v", diags)
	}

	return listValuable, nil
}

func (t KubernetesListType) ValueType(ctx context.Context) attr.Value {
	return KubernetesListValue{}
}

func (t KubernetesListType) ValueFromUnstructured(ctx context.Context, path path.Path, obj interface{}) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	sliceObj, ok := obj.([]interface{})
	if !ok {
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type",
			fmt.Sprintf("Expected list of items, got %T", obj),
		))
		return nil, diags
	}

	elems := make([]attr.Value, 0, len(sliceObj))
	for i, value := range sliceObj {
		elemPath := path.AtListIndex(i)

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
		elems = append(elems, elem)
	}

	baseList, listDiags := basetypes.NewListValue(t.ElemType, elems)
	diags.Append(listDiags...)
	result, listDiags := t.ValueFromList(ctx, baseList)
	diags.Append(listDiags...)

	return result, diags
}

func (t KubernetesListType) SchemaType(ctx context.Context, required bool) (schema.Attribute, error) {
	elem := t.ElementType()
	if objectElem, ok := elem.(KubernetesObjectType); ok {
		attributes, err := objectElem.SchemaAttributes(ctx, required)
		if err != nil {
			return nil, err
		}
		return schema.ListNestedAttribute{
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
		return schema.ListAttribute{
			Required:    required,
			Optional:    !required,
			Computed:    false,
			CustomType:  t,
			ElementType: elem,
		}, nil
	}
}

func ListFromOpenApi(root *spec3.OpenAPI, openapi spec.Schema, path []string) (KubernetesType, error) {
	items := openapi.Items.Schema
	if items == nil {
		return nil, fmt.Errorf("expected map of items at %s", strings.Join(path, ""))
	}

	elemType, err := OpenApiToTfType(root, *items, append(path, "[*]"))
	if err != nil {
		return nil, err
	}

	return KubernetesListType{ListType: basetypes.ListType{ElemType: elemType}}, nil
}

var _ basetypes.ListTypable = KubernetesListType{}
var _ KubernetesType = KubernetesListType{}

type KubernetesListValue struct {
	basetypes.ListValue
}

func (v KubernetesListValue) ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	elems := v.ListValue.Elements()
	result := make([]interface{}, 0, len(elems))
	for i, elem := range elems {
		elemPath := path.AtListIndex(i)
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
		result = append(result, elemObj)
	}
	return result, nil
}

func (v KubernetesListValue) Equal(o attr.Value) bool {
	other, ok := o.(KubernetesListValue)
	if !ok {
		return false
	}
	return v.ListValue.Equal(other.ListValue)
}

func (v KubernetesListValue) Type(ctx context.Context) attr.Type {
	return KubernetesListType{ListType: basetypes.ListType{ElemType: v.ElementType(ctx)}}
}

func (v *KubernetesListValue) FillNulls(ctx context.Context, path path.Path, config attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics

	var kubernetesConfig *KubernetesListValue
	switch config := config.(type) {
	case basetypes.ListValue:
		baseConfig, diags := v.Type(ctx).(KubernetesListType).ValueFromList(ctx, config)
		if diags.HasError() {
			return diags
		}
		kubernetesConfig = baseConfig.(*KubernetesListValue)
	case *KubernetesListValue:
		kubernetesConfig = config
	default:
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type",
			fmt.Sprintf("Expected ListValue, got %T", config),
		))
		return diags
	}

	configElements := kubernetesConfig.Elements()
	if v.IsNull() && !kubernetesConfig.IsNull() && len(configElements) == 0 {
		v.ListValue, diags = basetypes.NewListValue(v.ElementType(ctx), []attr.Value{})
	} else if !v.IsNull() && kubernetesConfig.IsNull() && len(v.Elements()) == 0 {
		v.ListValue = basetypes.NewListNull(v.ElementType(ctx))
	} else {
		for i, v := range v.Elements() {
			if kubernetesValue, ok := v.(KubernetesValue); ok {
				kubernetesValue.FillNulls(ctx, path.AtListIndex(i), configElements[i])
			}
		}
	}

	return diags
}

var _ basetypes.ListValuable = KubernetesListValue{}
var _ KubernetesValue = &KubernetesListValue{}
