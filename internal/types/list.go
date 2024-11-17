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
	diffvalue "sigs.k8s.io/structured-merge-diff/v4/value"
)

type KubernetesListType struct {
	basetypes.ListType

	Keys []string
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
	value := KubernetesListValue{ListValue: in, keys: t.Keys}
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

func (t KubernetesListType) ValueFromUnstructured(ctx context.Context, path path.Path, fields *fieldpath.Set, obj interface{}) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj == nil {
		obj = make([]interface{}, 0)
	}

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

		var p fieldpath.PathElement
		if t.Keys != nil {
			key := make(diffvalue.FieldList, 0, len(t.Keys))
			obj := value.(map[string]interface{})
			for _, k := range t.Keys {
				v := diffvalue.NewValueInterface(obj[k])
				key = append(key, diffvalue.Field{Name: k, Value: v})
			}
			p = fieldpath.PathElement{Key: &key}
		} else {
			p = fieldpath.PathElement{Index: &i}
		}

		if kubernetesElemType, ok := t.ElemType.(KubernetesType); ok {
			if fields == nil || fields.Members.Has(p) {
				elem, attrDiags = kubernetesElemType.ValueFromUnstructured(ctx, elemPath, nil, value)
			} else if childFields, found := fields.Children.Get(p); found {
				elem, attrDiags = kubernetesElemType.ValueFromUnstructured(ctx, elemPath, childFields, value)
			} else {
				continue
			}
		} else {
			if fields == nil || fields.Members.Has(p) {
				elem, attrDiags = primitiveFromUnstructured(ctx, elemPath, t.ElemType, value)
			} else {
				continue
			}
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

	extensions := openapi.VendorExtensible.Extensions
	listType := extensions["x-kubernetes-list-type"].(string)
	var keys []string
	if listType == "map" {
		for _, k := range extensions["x-kubernetes-list-map-keys"].([]interface{}) {
			keys = append(keys, k.(string))
		}
	}

	elemType, err := OpenApiToTfType(root, *items, append(path, "[*]"))
	if err != nil {
		return nil, err
	}

	return KubernetesListType{ListType: basetypes.ListType{ElemType: elemType}, Keys: keys}, nil
}

var _ basetypes.ListTypable = KubernetesListType{}
var _ KubernetesType = KubernetesListType{}

type KubernetesListValue struct {
	basetypes.ListValue

	keys []string
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
	return KubernetesListType{ListType: basetypes.ListType{ElemType: v.ElementType(ctx)}, Keys: v.keys}
}

func (v KubernetesListValue) ManagedFields(ctx context.Context, path path.Path, fields *fieldpath.Set, pe *fieldpath.PathElement) diag.Diagnostics {
	var diags diag.Diagnostics

	fields = fields.Children.Descend(*pe)
	for i, elem := range v.Elements() {
		if elem.IsNull() {
			continue
		}

		fieldPath := path.AtListIndex(i)

		var pathElem fieldpath.PathElement
		if v.keys != nil {
			key := make(diffvalue.FieldList, 0, len(v.keys))

			obj := elem.(*KubernetesObjectValue)
			unstructured, objDiags := obj.ToUnstructured(ctx, path)
			diags.Append(objDiags...)
			if objDiags.HasError() {
				continue
			}

			unstructuredObj := unstructured.(map[string]interface{})
			for _, k := range v.keys {
				v := diffvalue.NewValueInterface(unstructuredObj[k])
				key = append(key, diffvalue.Field{Name: k, Value: v})
			}
			pathElem = fieldpath.PathElement{Key: &key}
		} else {
			pathElem = fieldpath.PathElement{Index: &i}
		}

		if kubernetesAttr, ok := elem.(KubernetesValue); ok {
			diags.Append(kubernetesAttr.ManagedFields(ctx, fieldPath, fields, &pathElem)...)
		} else {
			fields.Insert([]fieldpath.PathElement{pathElem})
		}
	}

	return diags
}

var _ basetypes.ListValuable = KubernetesListValue{}
var _ KubernetesValue = KubernetesListValue{}
