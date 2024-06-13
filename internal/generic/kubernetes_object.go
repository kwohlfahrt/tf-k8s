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
	strcase "github.com/stoewer/go-strcase"
)

type KubernetesType interface {
	attr.Type

	SchemaType(ctx context.Context, isDatasource, isRequired bool) (schema.Attribute, error)
	ValueFromUnstructured(ctx context.Context, path path.Path, obj interface{}) (attr.Value, diag.Diagnostics)
}

type KubernetesObjectType struct {
	basetypes.ObjectType

	fieldNames     map[string]string
	requiredFields map[string]bool
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

func (t KubernetesObjectType) ValueFromUnstructured(ctx context.Context, path path.Path, obj interface{}) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	mapObj, ok := obj.(map[string]interface{})
	if !ok {
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type",
			fmt.Sprintf("Expected map of properties, got %T", obj),
		))
		return nil, diags
	}

	attributes := make(map[string]attr.Value, len(mapObj))
	for k, attrType := range t.AttrTypes {
		fieldPath := path.AtName(k)
		fieldName, found := t.fieldNames[k]
		if !found {
			diags.Append(diag.NewAttributeErrorDiagnostic(
				fieldPath, "Unexpected field",
				"Field does not have a mapping to a Kubernetes property. This is a provider-internal error, please report it!",
			))
			attributes[k] = newNull(ctx, attrType)
			continue
		}
		value, found := mapObj[fieldName]
		if !found {
			attributes[k] = newNull(ctx, attrType)
			continue
		}

		var attr attr.Value
		var attrDiags diag.Diagnostics

		if kubernetesAttrType, ok := attrType.(KubernetesType); ok {
			attr, attrDiags = kubernetesAttrType.ValueFromUnstructured(ctx, fieldPath, value)
		} else {
			attr, attrDiags = primitiveFromUnstructured(ctx, fieldPath, attrType, value)
		}
		diags.Append(attrDiags...)
		if attrDiags.HasError() {
			continue
		}
		attributes[k] = attr
	}

	baseObj, objDiags := basetypes.NewObjectValue(t.AttrTypes, attributes)
	diags.Append(objDiags...)
	result, objDiags := t.ValueFromObject(ctx, baseObj)
	diags.Append(objDiags...)

	return result, diags
}

func (t KubernetesObjectType) SchemaAttributes(ctx context.Context, isDatasource bool, isRequired bool) (map[string]schema.Attribute, error) {
	attributes := make(map[string]schema.Attribute, len(t.AttrTypes))
	for k, attr := range t.AttrTypes {
		isRequired := t.requiredFields[k]
		var schemaType schema.Attribute
		var err error

		if kubernetesAttr, ok := attr.(KubernetesType); ok {
			schemaType, err = kubernetesAttr.SchemaType(ctx, isDatasource, isRequired)
		} else {
			schemaType, err = primitiveSchemaType(ctx, attr, isDatasource, isRequired)
		}

		if err != nil {
			return nil, err
		}
		attributes[k] = schemaType
	}
	return attributes, nil
}

func (t KubernetesObjectType) SchemaType(ctx context.Context, isDatasource bool, isRequired bool) (schema.Attribute, error) {
	computed := isDatasource
	optional := !isDatasource && !isRequired
	required := !isDatasource && isRequired

	attributes, err := t.SchemaAttributes(ctx, isDatasource, isRequired)
	if err != nil {
		return nil, err
	}

	return schema.SingleNestedAttribute{
		Required:   required,
		Optional:   optional,
		Computed:   computed,
		Attributes: attributes,
		CustomType: t,
	}, nil
}

func ObjectFromOpenApi(openapi map[string]interface{}, path []string) (KubernetesType, error) {
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

		attribute, err := openApiToTfType(property, attrPath)
		if err != nil {
			return nil, err
		}
		fieldName := strcase.SnakeCase(k)
		attrTypes[fieldName] = attribute
		fieldNames[fieldName] = k
	}

	rawRequired, found := openapi["required"]
	if !found {
		rawRequired = []interface{}{}
	}
	required, ok := rawRequired.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected list of required fields at %s", strings.Join(path, ""))
	}
	requiredFields := make(map[string]bool, len(required))
	for i, field := range required {
		fieldName, ok := field.(string)
		if !ok {
			indexPath := append(path, fmt.Sprintf("[%d]", i))
			return nil, fmt.Errorf("expected name of required fields at %s", strings.Join(indexPath, ""))
		}
		requiredFields[strcase.SnakeCase(fieldName)] = true
	}

	return KubernetesObjectType{
		ObjectType:     basetypes.ObjectType{AttrTypes: attrTypes},
		fieldNames:     fieldNames,
		requiredFields: requiredFields,
	}, nil
}

func openApiToTfType(openapi map[string]interface{}, path []string) (attr.Type, error) {
	switch ty := openapi["type"]; ty {
	case "object":
		if _, isMap := openapi["additionalProperties"]; isMap {
			return MapFromOpenApi(openapi, path)
		} else {
			return ObjectFromOpenApi(openapi, path)
		}
	case "array":
		return ListFromOpenApi(openapi, path)
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
var _ KubernetesType = KubernetesObjectType{}

type KubernetesValue interface {
	attr.Value

	ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics)
}

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

func (v KubernetesObjectValue) ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	attributes := v.Attributes()
	result := make(map[string]interface{}, len(attributes))
	for k, attr := range attributes {
		if attr.IsNull() {
			continue
		}

		fieldPath := path.AtName(k)
		fieldName, found := v.fieldNames[k]
		if !found {
			diags.Append(diag.NewAttributeErrorDiagnostic(
				fieldPath, "Unexpected field",
				"Field does not have a mapping to a Kubernetes property. This is a provider-internal error, please report it!",
			))
			continue
		}
		var attrObj interface{}
		var attrDiags diag.Diagnostics
		if kubernetesAttr, ok := attr.(KubernetesValue); ok {
			attrObj, attrDiags = kubernetesAttr.ToUnstructured(ctx, fieldPath)
		} else {
			attrObj, attrDiags = primitiveToUnstructured(ctx, fieldPath, attr)
		}
		diags.Append(attrDiags...)
		if attrDiags.HasError() {
			continue
		}

		result[fieldName] = attrObj
	}

	return result, diags
}

var _ basetypes.ObjectValuable = KubernetesObjectValue{}
var _ KubernetesValue = KubernetesObjectValue{}
