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

type KubernetesUnionType struct {
	basetypes.DynamicType

	Members []attr.Type
}

func (t KubernetesUnionType) SchemaType(ctx context.Context, opts SchemaOptions, isRequired bool) (schema.Attribute, error) {
	return schema.DynamicAttribute{
		Required:   isRequired,
		Optional:   !isRequired,
		Computed:   false,
		CustomType: t,
	}, nil
}

func (t KubernetesUnionType) ValueFromUnstructured(
	ctx context.Context,
	path path.Path,
	fields *fieldpath.Set,
	obj interface{},
) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	for _, member := range t.Members {
		var val attr.Value
		var memberDiags diag.Diagnostics
		if kubernetesMember, ok := member.(KubernetesType); ok {
			val, memberDiags = kubernetesMember.ValueFromUnstructured(ctx, path, fields, obj)
		} else {
			val, memberDiags = primitiveFromUnstructured(ctx, path, member, obj)
		}
		if !memberDiags.HasError() {
			dynamicVal, dynamicDiags := t.ValueFromDynamic(ctx, basetypes.NewDynamicValue(val))
			memberDiags.Append(dynamicDiags...)
			return dynamicVal, memberDiags
		}
		diags.Append(memberDiags...)
	}
	return nil, diags
}

func (t KubernetesUnionType) Equal(o attr.Type) bool {
	other, ok := o.(KubernetesUnionType)
	if !ok {
		return false
	}
	return t.DynamicType.Equal(other.DynamicType)
}

func (t KubernetesUnionType) String() string {
	return "KubernetesUnionType"
}

func (t KubernetesUnionType) ValueFromDynamic(ctx context.Context, in basetypes.DynamicValue) (basetypes.DynamicValuable, diag.Diagnostics) {
	value := KubernetesUnionValue{
		DynamicValue: in,
		MemberTypes:  t.Members,
	}
	return &value, nil
}

func (t KubernetesUnionType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.DynamicType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	dynamicValue, ok := attrValue.(basetypes.DynamicValue)
	if !ok {
		return nil, fmt.Errorf("expected DynamicValue, got %T", attrValue)
	}

	dynamicValuable, diags := t.ValueFromDynamic(ctx, dynamicValue)
	if diags.HasError() {
		return nil, fmt.Errorf("error converting DynamicValue to DynamicValuable: %v", diags)
	}

	return dynamicValuable, nil
}

func (t KubernetesUnionType) ValueType(ctx context.Context) attr.Value {
	return KubernetesUnionValue{
		MemberTypes: t.Members,
	}
}

var _ basetypes.DynamicTypable = KubernetesUnionType{}
var _ KubernetesType = KubernetesUnionType{}

type KubernetesUnionValue struct {
	basetypes.DynamicValue

	MemberTypes []attr.Type
}

func (v KubernetesUnionValue) Equal(o attr.Value) bool {
	other, ok := o.(KubernetesUnionValue)
	if !ok {
		return false
	}
	return v.DynamicValue.Equal(other)
}

func (v KubernetesUnionValue) ToUnstructured(ctx context.Context, path path.Path) (interface{}, diag.Diagnostics) {
	val := v.DynamicValue.UnderlyingValue()
	if kubernetesVal, ok := val.(KubernetesValue); ok {
		return kubernetesVal.ToUnstructured(ctx, path)
	} else {
		return primitiveToUnstructured(ctx, path, val)
	}
}

func (v KubernetesUnionValue) Type(context.Context) attr.Type {
	return KubernetesUnionType{Members: v.MemberTypes}
}

func (v KubernetesUnionValue) ManagedFields(ctx context.Context, path path.Path, fields *fieldpath.Set, pe *fieldpath.PathElement) diag.Diagnostics {
	val := v.DynamicValue.UnderlyingValue()
	if kubernetesVal, ok := val.(KubernetesValue); ok {
		return kubernetesVal.ManagedFields(ctx, path, fields, pe)
	} else {
		fields.Insert([]fieldpath.PathElement{*pe})
		return nil
	}
}

func (v KubernetesUnionValue) Validate(ctx context.Context, path path.Path) diag.Diagnostics {
	// TODO
	return nil
}

var _ basetypes.DynamicValuable = KubernetesUnionValue{}
var _ KubernetesValue = KubernetesUnionValue{}

func UnionFromOpenApi(root *spec3.OpenAPI, openapis spec.Schema, path []string) (KubernetesType, error) {
	members := openapis.OneOf
	if members == nil {
		members = openapis.AnyOf
	}
	if members == nil {
		return nil, fmt.Errorf("expected union type at %s", strings.Join(path, ""))
	}

	memberTypes := make([]attr.Type, 0, len(members))
	for _, member := range members {
		memberType, err := OpenApiToTfType(root, member, path)
		if err != nil {
			return nil, err
		}
		memberTypes = append(memberTypes, memberType)
	}
	return KubernetesUnionType{Members: memberTypes}, nil
}
