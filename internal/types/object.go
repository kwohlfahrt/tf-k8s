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
	strcase "github.com/stoewer/go-strcase"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type KubernetesType interface {
	attr.Type

	SchemaType(ctx context.Context, isRequired bool) (schema.Attribute, error)
	ValueFromUnstructured(ctx context.Context, path path.Path, obj interface{}) (attr.Value, diag.Diagnostics)
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
	return &value, nil
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
	if obj == nil {
		obj = make(map[string]interface{}, 0)
	}

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
		if !found || value == nil {
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
			attributes[k] = newNull(ctx, attrType)
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

func (t KubernetesObjectType) SchemaAttributes(ctx context.Context, isRequired bool) (map[string]schema.Attribute, error) {
	attributes := make(map[string]schema.Attribute, len(t.AttrTypes))
	for k, attr := range t.AttrTypes {
		isRequired := t.RequiredFields[k]
		var schemaType schema.Attribute
		var err error

		if kubernetesAttr, ok := attr.(KubernetesType); ok {
			schemaType, err = kubernetesAttr.SchemaType(ctx, isRequired)
		} else {
			schemaType, err = primitiveSchemaType(ctx, attr, isRequired)
		}

		if err != nil {
			return nil, err
		}
		attributes[k] = schemaType
	}
	return attributes, nil
}

func (t KubernetesObjectType) SchemaType(ctx context.Context, required bool) (schema.Attribute, error) {
	attributes, err := t.SchemaAttributes(ctx, required)
	if err != nil {
		return nil, err
	}

	return schema.SingleNestedAttribute{
		Required:   required,
		Optional:   !required,
		Computed:   false,
		Attributes: attributes,
		CustomType: t,
	}, nil
}

func ObjectFromOpenApi(root *spec3.OpenAPI, openapi spec.Schema, path []string) (KubernetesType, error) {
	properties := openapi.Properties

	attrTypes := make(map[string]attr.Type, len(properties))
	fieldNames := make(map[string]string, len(properties))
	for k, property := range properties {
		attrPath := append(path, fmt.Sprintf(".%s", k))
		attribute, err := OpenApiToTfType(root, property, attrPath)
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

func OpenApiToTfType(root *spec3.OpenAPI, openapi spec.Schema, path []string) (attr.Type, error) {
	if pointer := openapi.Ref.GetPointer(); !pointer.IsEmpty() {
		// TODO: Special-case ObjectMeta
		maybeSchema, _, err := pointer.Get(root)
		if err != nil {
			return nil, err
		}
		schema, ok := maybeSchema.(*spec.Schema)
		if !ok {
			return nil, fmt.Errorf("expected schema at ref %s, got %T", strings.Join(path, ""), maybeSchema)
		}
		return OpenApiToTfType(root, *schema, path)
	}
	if len(openapi.Type) == 0 {
		switch {
		case len(openapi.AllOf) == 1:
			return OpenApiToTfType(root, openapi.AllOf[0], path)
		case len(openapi.OneOf) == 1:
			return OpenApiToTfType(root, openapi.OneOf[0], path)
		case len(openapi.OneOf) > 1:
			return UnionFromOpenApi(root, openapi, path)
		default:
			return nil, fmt.Errorf("expected concrete or union type at %s", strings.Join(path, ""))
		}
	}
	var ty string
	if len(openapi.Type) == 1 {
		ty = openapi.Type[0]
	} else {
		return nil, fmt.Errorf("expected exactly one type at %s", strings.Join(path, ""))
	}

	switch ty {
	case "object":
		if openapi.AdditionalProperties != nil {
			return MapFromOpenApi(root, openapi, path)
		} else {
			return ObjectFromOpenApi(root, openapi, path)
		}
	case "array":
		return ListFromOpenApi(root, openapi, path)
	case "string":
		return basetypes.StringType{}, nil
	case "integer":
		return basetypes.Int64Type{}, nil
	case "boolean":
		return basetypes.BoolType{}, nil
	case "number":
		return basetypes.NumberType{}, nil
	default:
		return nil, fmt.Errorf("unrecognized type at %s: %s", strings.Join(path, ""), ty)
	}
}

var _ basetypes.ObjectTypable = KubernetesObjectType{}
var _ KubernetesType = KubernetesObjectType{}

type KubernetesValue interface {
	attr.Value

	ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics)
	FillNulls(ctx context.Context, path path.Path, config attr.Value) diag.Diagnostics
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
		if attr.IsNull() || attr.IsUnknown() {
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

func (v *KubernetesObjectValue) FillNulls(ctx context.Context, path path.Path, config attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics

	var kubernetesConfig *KubernetesObjectValue
	switch config := config.(type) {
	case basetypes.ObjectValue:
		baseConfig, diags := v.Type(ctx).(KubernetesObjectType).ValueFromObject(ctx, config)
		if diags.HasError() {
			return diags
		}
		kubernetesConfig = baseConfig.(*KubernetesObjectValue)
	case *KubernetesObjectValue:
		kubernetesConfig = config
	default:
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type",
			fmt.Sprintf("Expected ObjectValue, got %T", config),
		))
		return diags
	}

	configAttrs := kubernetesConfig.Attributes()
	for k, fieldValue := range v.Attributes() {
		fieldPath := path.AtName(k)
		if kubernetesFieldValue, ok := fieldValue.(KubernetesValue); ok {
			attr := configAttrs[k]
			diags.Append(kubernetesFieldValue.FillNulls(ctx, fieldPath, attr)...)
		}
	}

	return diags
}

var _ basetypes.ObjectValuable = KubernetesObjectValue{}
var _ KubernetesValue = &KubernetesObjectValue{}
