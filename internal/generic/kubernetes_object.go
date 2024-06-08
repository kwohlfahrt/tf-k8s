package generic

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	strcase "github.com/stoewer/go-strcase"
)

type KubernetesObjectType struct {
	basetypes.ObjectType

	fieldNames map[string]string
}

func (t KubernetesObjectType) Equal(o attr.Type) bool {
	other, ok := o.(KubernetesObjectType)
	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t KubernetesObjectType) String() string {
	return "KubernetesObjectType"
}

func (t KubernetesObjectType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	value := KubernetesObjectValue{
		ObjectValue: in,
		fieldNames:  t.fieldNames,
	}
	return value, nil
}

func (t KubernetesObjectType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.ObjectType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	objectValue, ok := attrValue.(basetypes.ObjectValue)
	if !ok {
		return nil, fmt.Errorf("expected ObjectValue, got %T", attrValue)
	}

	objectValuable, diags := t.ValueFromObject(ctx, objectValue)
	if diags.HasError() {
		return nil, fmt.Errorf("error converting ObjectValue to ObjectValuable: %v", diags)
	}

	return objectValuable, nil
}

func (t KubernetesObjectType) ValueType(ctx context.Context) attr.Value {
	return KubernetesObjectValue{
		fieldNames: t.fieldNames,
	}
}

func FromOpenApi(openapi map[string]interface{}, path []string) (basetypes.ObjectTypable, error) {
	properties, ok := openapi["properties"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map of properties at %s", strings.Join(path, ""))
	}

	attrTypes := make(map[string]attr.Type, len(properties))
	fieldNames := make(map[string]string, len(properties))
	for k, v := range properties {
		attrPath := append(path, fmt.Sprintf(".%s", k))
		property, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object at %s", strings.Join(attrPath, ""))
		}

		attribute, err := openApiToTfType(property, append(path, k))
		if err != nil {
			return nil, err
		}
		fieldName := strcase.SnakeCase(k)
		attrTypes[fieldName] = attribute
		fieldNames[fieldName] = k
	}

	return KubernetesObjectType{
		ObjectType: basetypes.ObjectType{AttrTypes: attrTypes},
		fieldNames: fieldNames,
	}, nil
}

func openApiToTfType(openapi map[string]interface{}, path []string) (attr.Type, error) {
	switch ty := openapi["type"]; ty {
	case "object":
		if rawItems, isMap := openapi["additionalProperties"]; isMap {
			items, ok := rawItems.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected additionalProperties object at %s", strings.Join(path, ""))
			}
			attribute, err := openApiToTfType(items, append(path, "[*]"))
			if err != nil {
				return nil, err
			}
			return basetypes.MapType{ElemType: attribute}, nil
		} else {
			attribute, err := FromOpenApi(openapi, path)
			if err != nil {
				return nil, err
			}
			return attribute, nil
		}
	case "array":
		items, ok := openapi["items"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("object %s has no items", strings.Join(path, ""))
		}

		attribute, err := openApiToTfType(items, append(path, "[*]"))
		if err != nil {
			return nil, err
		}
		return basetypes.ListType{ElemType: attribute}, nil
	case "string":
		return basetypes.StringType{}, nil
	case "integer":
		return basetypes.Int64Type{}, nil
	case "boolean":
		return basetypes.BoolType{}, nil
	default:
		return nil, fmt.Errorf("unrecognized type at %s: %s", strings.Join(path, ""), ty)
	}
}

var _ basetypes.ObjectTypable = KubernetesObjectType{}

type KubernetesObjectValue struct {
	basetypes.ObjectValue

	fieldNames map[string]string
}

func (v KubernetesObjectValue) Equal(o attr.Value) bool {
	other, ok := o.(KubernetesObjectValue)
	if !ok {
		return false
	}
	return v.ObjectValue.Equal(other.ObjectValue)
}

func (v KubernetesObjectValue) Type(ctx context.Context) attr.Type {
	return KubernetesObjectType{
		ObjectType: basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
		fieldNames: v.fieldNames,
	}
}

var _ basetypes.ObjectValuable = KubernetesObjectValue{}
