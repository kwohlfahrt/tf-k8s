package generic

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
	return value, nil
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

func (t KubernetesListType) SchemaType(ctx context.Context, isDatasource bool, isRequired bool) (schema.Attribute, error) {
	computed := isDatasource
	optional := !isDatasource && !isRequired
	required := !isDatasource && isRequired

	elem := t.ElementType()
	if objectElem, ok := elem.(KubernetesObjectType); ok {
		attributes, err := objectElem.SchemaAttributes(ctx, isDatasource, isRequired)
		if err != nil {
			return nil, err
		}
		return schema.ListNestedAttribute{
			Required:   required,
			Optional:   optional,
			Computed:   computed,
			CustomType: t,
			NestedObject: schema.NestedAttributeObject{
				Attributes: attributes,
				CustomType: objectElem,
			},
		}, nil
	} else {
		return schema.ListAttribute{
			Required:    required,
			Optional:    optional,
			Computed:    computed,
			CustomType:  t,
			ElementType: elem,
		}, nil
	}
}

func ListFromOpenApi(openapi map[string]interface{}, path []string) (KubernetesType, error) {
	items, ok := openapi["items"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map of items at %s", strings.Join(path, ""))
	}

	elemType, err := openApiToTfType(items, append(path, "[*]"))
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

var _ basetypes.ListValuable = KubernetesListValue{}
var _ KubernetesValue = KubernetesListValue{}
