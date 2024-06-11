package generic

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
			Required: required,
			Optional: optional,
			Computed: computed,
			NestedObject: schema.NestedAttributeObject{
				Attributes: attributes,
				CustomType: objectElem,
			},
		}, nil
	} else {
		return schema.ListAttribute{Required: required, Optional: optional, Computed: computed, ElementType: elem}, nil
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

type KubernetesListValue struct {
	basetypes.ListValue
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
