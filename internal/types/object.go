package types

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	strcase "github.com/stoewer/go-strcase"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type KubernetesType interface {
	attr.Type

	SchemaType(ctx context.Context, isDatasource, isRequired bool) (schema.Attribute, error)
	ValueFromUnstructured(ctx context.Context, path path.Path, obj interface{}) (attr.Value, diag.Diagnostics)
	Codegen(builder *strings.Builder)
}

type KubernetesObjectType struct {
	basetypes.ObjectType

	FieldNames     map[string]string
	RequiredFields map[string]bool
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
		fieldNames:  t.FieldNames,
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
		fieldNames: t.FieldNames,
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
		fieldName, found := t.FieldNames[k]
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
		isRequired := t.RequiredFields[k]
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
func (t KubernetesObjectType) Codegen(builder *strings.Builder) {
	builder.WriteString("types.KubernetesObjectType{")
	builder.WriteString("ObjectType: basetypes.ObjectType{")
	builder.WriteString("AttrTypes: map[string]attr.Type{")
	for k, attr := range t.ObjectType.AttrTypes {
		builder.WriteString(strconv.Quote(k))
		builder.WriteString(": ")
		if kubernetesAttr, ok := attr.(KubernetesType); ok {
			kubernetesAttr.Codegen(builder)
		} else {
			// TODO
			primitiveCodegen(attr, builder)
		}
		builder.WriteString(",")
	}
	builder.WriteString("},")
	builder.WriteString("},")
	builder.WriteString("RequiredFields: map[string]bool{")
	for k, v := range t.RequiredFields {
		builder.WriteString(fmt.Sprintf("%s: %t,", strconv.Quote(k), v))
	}
	builder.WriteString("},")
	builder.WriteString("FieldNames: map[string]string{")
	for k, v := range t.FieldNames {
		builder.WriteString(fmt.Sprintf("%s: %s,", strconv.Quote(k), strconv.Quote(v)))
	}
	builder.WriteString("},")
	builder.WriteString("}")
}

func ObjectFromOpenApi(openapi spec.Schema, path []string) (KubernetesType, error) {
	properties := openapi.Properties
	if properties == nil {
		return nil, fmt.Errorf("expected properties at %s", strings.Join(path, ""))
	}

	attrTypes := make(map[string]attr.Type, len(properties))
	fieldNames := make(map[string]string, len(properties))
	for k, property := range properties {
		attrPath := append(path, fmt.Sprintf(".%s", k))
		attribute, err := openApiToTfType(property, attrPath)
		if err != nil {
			return nil, err
		}
		fieldName := strcase.SnakeCase(k)
		attrTypes[fieldName] = attribute
		fieldNames[fieldName] = k
	}

	required := openapi.Required
	requiredFields := make(map[string]bool, len(required))
	for _, fieldName := range required {
		requiredFields[strcase.SnakeCase(fieldName)] = true
	}

	return KubernetesObjectType{
		ObjectType:     basetypes.ObjectType{AttrTypes: attrTypes},
		FieldNames:     fieldNames,
		RequiredFields: requiredFields,
	}, nil
}

func openApiToTfType(openapi spec.Schema, path []string) (attr.Type, error) {
	if len(openapi.Type) != 1 {
		return nil, fmt.Errorf("expected exactly one type at %s", strings.Join(path, ""))
	}

	switch ty := openapi.Type[0]; ty {
	case "object":
		if openapi.AdditionalProperties != nil {
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
		FieldNames: v.fieldNames,
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

func DynamicObjectFromUnstructured(ctx context.Context, path path.Path, val map[string]interface{}) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrValues := make(map[string]attr.Value, len(val))
	attrTypes := make(map[string]attr.Type, len(val))
	fieldNames := make(map[string]string, len(val))
	for k, v := range val {
		fieldName := strcase.SnakeCase(k)
		fieldPath := path.AtName(fieldName)
		var attrValue attr.Value
		var attrDiags diag.Diagnostics
		switch v := v.(type) {
		case map[string]interface{}:
			attrValue, attrDiags = DynamicObjectFromUnstructured(ctx, fieldPath, v)
		case []interface{}:
			attrValue, attrDiags = dynamicTupleFromUnstructured(ctx, fieldPath, v)
		default:
			attrValue, attrDiags = dynamicPrimitiveFromUnstructured(ctx, fieldPath, v)
		}
		diags.Append(attrDiags...)
		if attrDiags.HasError() {
			continue
		}

		fieldNames[fieldName] = k
		attrValues[fieldName] = attrValue
		attrTypes[fieldName] = attrValue.Type(ctx)
	}
	typ := KubernetesObjectType{
		ObjectType: basetypes.ObjectType{AttrTypes: attrTypes},
		FieldNames: fieldNames,
	}

	obj, objDiags := basetypes.NewObjectValue(attrTypes, attrValues)
	diags.Append(objDiags...)

	kubernetesObj, objDiags := typ.ValueFromObject(ctx, obj)
	diags.Append(objDiags...)
	return kubernetesObj, diags
}

var _ basetypes.ObjectValuable = KubernetesObjectValue{}
var _ KubernetesValue = KubernetesObjectValue{}
