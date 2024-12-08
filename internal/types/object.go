package types

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	strcase "github.com/stoewer/go-strcase"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type KubernetesType interface {
	attr.Type

	ValueFromUnstructured(ctx context.Context, path path.Path, fields *fieldpath.Set, obj interface{}) (attr.Value, diag.Diagnostics)
	Validate(ctx context.Context, path path.Path, value attr.Value, isDataSource bool) diag.Diagnostics
}

type KubernetesObjectType struct {
	basetypes.DynamicType

	AttrTypes      map[string]attr.Type
	FieldNames     map[string]string
	RequiredFields map[string]bool
}

func (t KubernetesObjectType) Equal(o attr.Type) bool {
	other, ok := o.(KubernetesObjectType)
	if !ok {
		return false
	}

	return t.DynamicType.Equal(other.DynamicType)
}

func (t KubernetesObjectType) String() string {
	return "KubernetesObjectType"
}

func (t KubernetesObjectType) ValueFromDynamic(ctx context.Context, in basetypes.DynamicValue) (basetypes.DynamicValuable, diag.Diagnostics) {
	var diags diag.Diagnostics
	value := KubernetesObjectValue{
		DynamicValue:   in,
		attrTypes:      t.AttrTypes,
		fieldNames:     t.FieldNames,
		requiredFields: t.RequiredFields,
	}
	if in.IsNull() || in.IsUnderlyingValueNull() || in.IsUnknown() || in.IsUnderlyingValueUnknown() {
		return value, diags
	}

	underlying := in.UnderlyingValue()
	if _, ok := underlying.(basetypes.ObjectValue); !ok {
		diags.Append(diag.NewErrorDiagnostic("Unexpected value type", fmt.Sprintf("Expected ObjectValue, got %T", underlying)))
		return nil, diags
	}
	return value, diags
}

func (t KubernetesObjectType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	var obj basetypes.ObjectValue
	switch {
	case in.IsNull():
		obj = basetypes.NewObjectNull(t.AttrTypes)
	case !in.IsKnown():
		obj = basetypes.NewObjectUnknown(t.AttrTypes)
	default:
		inObj := make(map[string]tftypes.Value)
		if err := in.As(&inObj); err != nil {
			return nil, err
		}
		attrs := make(map[string]attr.Value, len(inObj))
		attrTypes := make(map[string]attr.Type, len(inObj))
		for k, v := range inObj {
			var attrType attr.Type
			var found bool

			if attrType, found = t.AttrTypes[k]; !found {
				attrType = basetypes.DynamicType{}
			}

			attrValue, err := attrType.ValueFromTerraform(ctx, v)
			if err != nil {
				return nil, err
			}

			attrs[k] = attrValue
			attrTypes[k] = attrType
		}
		obj = basetypes.NewObjectValueMust(attrTypes, attrs)
	}

	kubernetesValue, _ := t.ValueFromDynamic(ctx, basetypes.NewDynamicValue(obj))
	return kubernetesValue, nil
}

func (t KubernetesObjectType) ValueType(ctx context.Context) attr.Value {
	return KubernetesObjectValue{
		attrTypes:      t.AttrTypes,
		fieldNames:     t.FieldNames,
		requiredFields: t.RequiredFields,
	}
}

func (t KubernetesObjectType) ValueFromUnstructured(
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
			fmt.Sprintf("Expected map of properties, got %T", obj),
		))
		return nil, diags
	}

	attributes := make(map[string]attr.Value, len(mapObj))
	attrTypes := make(map[string]attr.Type, len(mapObj))

	for k, attrType := range t.AttrTypes {
		fieldPath := path.AtName(k)
		fieldName, found := t.FieldNames[k]
		if !found {
			continue
		}

		var attr attr.Value
		var attrDiags diag.Diagnostics

		value, found := mapObj[fieldName]
		// Handle the parsing/datasource case, where we don't have a field-manager
		if fields == nil && !found {
			continue
		}

		p := fieldpath.PathElement{FieldName: &fieldName}
		if kubernetesAttrType, ok := attrType.(KubernetesType); ok {
			if fields == nil || fields.Members.Has(p) {
				attr, attrDiags = kubernetesAttrType.ValueFromUnstructured(ctx, fieldPath, nil, value)
			} else if childFields, found := fields.Children.Get(p); found {
				attr, attrDiags = kubernetesAttrType.ValueFromUnstructured(ctx, fieldPath, childFields, value)
			} else {
				continue
			}
		} else {
			if fields == nil || fields.Members.Has(p) {
				attr, attrDiags = primitiveFromUnstructured(ctx, fieldPath, attrType, value)
			} else {
				continue
			}
		}
		diags.Append(attrDiags...)
		if attrDiags.HasError() {
			continue
		}
		attributes[k] = attr
		attrTypes[k] = attr.Type(ctx)
	}

	baseObj, objDiags := basetypes.NewObjectValue(attrTypes, attributes)
	diags.Append(objDiags...)
	result, objDiags := t.ValueFromDynamic(ctx, basetypes.NewDynamicValue(baseObj))
	diags.Append(objDiags...)

	return result, diags
}

type SchemaTypeOpts struct {
	IsDataSource bool
}

func (t KubernetesObjectType) SchemaType(ctx context.Context, opts SchemaTypeOpts) (schema.Attribute, error) {
	return schema.DynamicAttribute{
		Required:   true,
		Optional:   false,
		Computed:   false,
		CustomType: t,
		Validators: []validator.Dynamic{t.Validator(ctx, opts.IsDataSource)},
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
		DynamicType:    basetypes.DynamicType{},
		AttrTypes:      attrTypes,
		FieldNames:     fieldNames,
		RequiredFields: requiredFields,
	}, nil
}

func (t KubernetesObjectType) Validate(ctx context.Context, path path.Path, in attr.Value, isDataSource bool) diag.Diagnostics {
	var diags diag.Diagnostics

	var attrs map[string]attr.Value
	switch in := in.(type) {
	case KubernetesObjectValue:
		if in.IsNull() || in.IsUnknown() || in.IsUnderlyingValueNull() || in.IsUnderlyingValueUnknown() {
			return diags
		}
		attrs = in.Attributes()
	case basetypes.DynamicValue:
		if in.IsNull() || in.IsUnknown() || in.IsUnderlyingValueNull() || in.IsUnderlyingValueUnknown() {
			return diags
		}
		if inObject, ok := in.UnderlyingValue().(basetypes.ObjectValue); ok {
			attrs = inObject.Attributes()
		} else {
			diags.Append(diag.NewAttributeErrorDiagnostic(
				path, "Unexpected value type", fmt.Sprintf("Expected dynamic object value, got %T", in),
			))
			return diags
		}
	default:
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Unexpected value type", fmt.Sprintf("Expected KubernetesObjectValue, got %T", in),
		))
		return diags
	}

	extraAttrs := make(map[string]bool, 0)
	missingAttrs := maps.Clone(t.RequiredFields)
	for field := range t.RequiredFields {
		missingAttrs[field] = true
	}

	for k, attr := range attrs {
		attrType, found := t.AttrTypes[k]
		if !found {
			extraAttrs[k] = true
		} else {
			delete(missingAttrs, k)
		}
		if kubernetesAttrType, ok := attrType.(KubernetesType); ok {
			diags.Append(kubernetesAttrType.Validate(ctx, path.AtMapKey(k), attr, isDataSource)...)
		}
	}

	if len(extraAttrs) > 0 {
		extraAttrNames := slices.Collect(maps.Keys(extraAttrs))
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Extra fields found", fmt.Sprintf("extra fields: %s", strings.Join(extraAttrNames, ", ")),
		))
	}

	if len(missingAttrs) > 0 && !isDataSource {
		missingAttrNames := slices.Collect(maps.Keys(missingAttrs))
		diags.Append(diag.NewAttributeErrorDiagnostic(
			path, "Missing required fields", fmt.Sprintf("missing fields: %s", strings.Join(missingAttrNames, ", ")),
		))
	}

	return diags
}

func (t KubernetesObjectType) Validator(ctx context.Context, isDataSource bool) validator.Dynamic {
	return objectValidator{
		t:            t,
		isDataSource: isDataSource,
	}
}

type objectValidator struct {
	t            KubernetesObjectType
	isDataSource bool
}

func (v objectValidator) Description(ctx context.Context) string {
	return "Object Validator"
}

func (v objectValidator) MarkdownDescription(ctx context.Context) string {
	return "Object Validator"
}

func (v objectValidator) ValidateDynamic(ctx context.Context, req validator.DynamicRequest, resp *validator.DynamicResponse) {
	resp.Diagnostics.Append(v.t.Validate(ctx, req.Path, req.ConfigValue, v.isDataSource)...)
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

	preserveUnknown := false
	if v, found := openapi.Extensions["x-kubernetes-preserve-unknown-fields"]; found {
		preserveUnknown = preserveUnknown || v.(bool)
	}

	if len(openapi.Type) == 0 {
		switch {
		case len(openapi.AllOf) == 1:
			return OpenApiToTfType(root, openapi.AllOf[0], path)
		case len(openapi.OneOf) == 1:
			return OpenApiToTfType(root, openapi.OneOf[0], path)
		case len(openapi.OneOf) > 1:
			return UnionFromOpenApi(root, openapi, path)
		case len(openapi.AnyOf) > 1:
			return UnionFromOpenApi(root, openapi, path)
		case preserveUnknown:
			return KubernetesUnknownType{}, nil
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

var _ basetypes.DynamicTypable = KubernetesObjectType{}
var _ KubernetesType = KubernetesObjectType{}

type KubernetesValue interface {
	attr.Value

	ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics)
	ManagedFields(ctx context.Context, path path.Path, fields *fieldpath.Set, pe *fieldpath.PathElement) diag.Diagnostics
}

type KubernetesObjectValue struct {
	basetypes.DynamicValue

	attrTypes      map[string]attr.Type
	fieldNames     map[string]string
	requiredFields map[string]bool
}

func (v KubernetesObjectValue) Equal(o attr.Value) bool {
	other, ok := o.(KubernetesObjectValue)
	if !ok {
		return false
	}
	return v.DynamicValue.Equal(other.DynamicValue)
}

func (v KubernetesObjectValue) Type(ctx context.Context) attr.Type {
	return KubernetesObjectType{
		DynamicType:    basetypes.DynamicType{},
		AttrTypes:      v.attrTypes,
		FieldNames:     v.fieldNames,
		RequiredFields: v.requiredFields,
	}
}

func (v KubernetesObjectValue) Attributes() map[string]attr.Value {
	return v.UnderlyingValue().(basetypes.ObjectValue).Attributes()
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

func (v KubernetesObjectValue) ManagedFields(ctx context.Context, path path.Path, fields *fieldpath.Set, pe *fieldpath.PathElement) diag.Diagnostics {
	var diags diag.Diagnostics

	if pe != nil {
		fields = fields.Children.Descend(*pe)
	}

	for k, attr := range v.Attributes() {
		if attr.IsNull() {
			continue
		}

		fieldPath := path.AtName(k)
		fieldName, found := v.fieldNames[k]
		if !found {
			continue
		}
		pathElem := fieldpath.PathElement{FieldName: &fieldName}
		if kubernetesAttr, ok := attr.(KubernetesValue); ok {
			diags.Append(kubernetesAttr.ManagedFields(ctx, fieldPath, fields, &pathElem)...)
		} else {
			fields.Insert([]fieldpath.PathElement{pathElem})
		}
	}
	return diags
}

var _ basetypes.DynamicValuable = KubernetesObjectValue{}
var _ KubernetesValue = KubernetesObjectValue{}
