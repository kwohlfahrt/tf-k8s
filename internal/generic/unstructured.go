package generic

import (
	"bytes"
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/value"
)

func StateToObject(ctx context.Context, state tfsdk.Plan, typeInfo TypeInfo) (*unstructured.Unstructured, diag.Diagnostics) {
	var diags diag.Diagnostics

	var stateValue basetypes.ObjectValue
	diags.Append(state.Get(ctx, &stateValue)...)
	if diags.HasError() {
		return nil, diags
	}

	value, valueDiags := typeInfo.Schema.ValueFromObject(ctx, stateValue)
	diags.Append(valueDiags...)
	if diags.HasError() {
		return nil, diags
	}
	objectValue := value.(*types.KubernetesObjectValue)
	obj, objDiags := objectValue.ToUnstructured(ctx, path.Empty())
	diags.Append(objDiags...)
	if diags.HasError() {
		return nil, diags
	}

	objMap := obj.(map[string]interface{})
	objMap["kind"] = typeInfo.Kind
	objMap["apiVersion"] = typeInfo.GroupVersionResource().GroupVersion().String()

	return &unstructured.Unstructured{Object: objMap}, diags
}

// A field extractor that doesn't depend on the schema, working around
// https://github.com/kubernetes/kubernetes/issues/128201
func Extract(in *unstructured.Unstructured, fieldManager string) (*unstructured.Unstructured, error) {
	var entry *v1.ManagedFieldsEntry
	for _, maybeEntry := range in.GetManagedFields() {
		if maybeEntry.Manager == fieldManager && maybeEntry.Operation == v1.ManagedFieldsOperationApply {
			entry = &maybeEntry
		}
	}
	if entry == nil {
		return &unstructured.Unstructured{}, nil
	}

	fieldSet := &fieldpath.Set{}
	err := fieldSet.FromJSON(bytes.NewReader(entry.FieldsV1.Raw))
	if err != nil {
		return nil, err
	}

	content, err := extractFields(in.UnstructuredContent(), fieldSet.Leaves())
	if err != nil {
		return nil, err
	}

	object := content.(map[string]interface{})
	object["apiVersion"] = in.GetAPIVersion()
	object["kind"] = in.GetKind()

	metadata := object["metadata"].(map[string]interface{})
	metadata["name"] = in.GetName()
	metadata["namespace"] = in.GetNamespace()

	return &unstructured.Unstructured{Object: content.(map[string]interface{})}, nil
}

func extractFields(in interface{}, fieldSet *fieldpath.Set) (out interface{}, err error) {
	isFieldName := false
	isKey := false
	isIndex := false
	isValue := false

	checkType := func(pe fieldpath.PathElement) {
		if pe.FieldName != nil {
			isFieldName = true
		}
		if pe.Key != nil {
			isKey = true
		}
		if pe.Index != nil {
			isIndex = true
		}
		if pe.Value != nil {
			isValue = true
		}
	}

	fieldSet.Members.Iterate(checkType)
	fieldSet.Children.Iterate(checkType)

	switch {
	case isFieldName:
		if isKey || isIndex || isValue {
			return nil, fmt.Errorf("got mixed key types")
		}
		inObj, ok := in.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map, got %T", inObj)
		}

		obj := make(map[string]interface{}, fieldSet.Children.Size()+fieldSet.Members.Size())
		fieldSet.Members.Iterate(func(pe fieldpath.PathElement) {
			obj[*pe.FieldName] = inObj[*pe.FieldName]
		})
		fieldSet.Children.Iterate(func(pe fieldpath.PathElement) {
			child, err := extractFields(inObj[*pe.FieldName], fieldSet.WithPrefix(pe))
			if err == nil {
				obj[*pe.FieldName] = child
			}
		})
		out = obj
	case isKey:
		if isFieldName || isIndex || isValue {
			return nil, fmt.Errorf("got mixed key types")
		}
		inObj, ok := in.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected list, got %T", inObj)
		}

		obj := make([]interface{}, 0, fieldSet.Children.Size()+fieldSet.Members.Size())

		// FIXME: Quadratic
		fieldSet.Members.Iterate(func(pe fieldpath.PathElement) {
			for _, v := range inObj {
				vObj := v.(map[string]interface{})
				keyFields := make([]value.Field, 0, len(*pe.Key))
				for _, field := range *pe.Key {
					keyFields = append(keyFields, value.Field{Name: field.Name, Value: value.NewValueInterface(vObj[field.Name])})
				}
				if pe.Key.Equals(keyFields) {
					obj = append(obj, v)
				}
			}
		})
		fieldSet.Children.Iterate(func(pe fieldpath.PathElement) {
			for _, v := range inObj {
				vObj := v.(map[string]interface{})
				keyFields := make([]value.Field, 0, len(*pe.Key))
				for _, field := range *pe.Key {
					keyFields = append(keyFields, value.Field{Name: field.Name, Value: value.NewValueInterface(vObj[field.Name])})
				}
				if pe.Key.Equals(keyFields) {
					child, err := extractFields(v, fieldSet.WithPrefix(pe))
					if err == nil {
						obj = append(obj, child)
					}
				}
			}
		})

		out = obj
	case isIndex:
		if isFieldName || isKey || isValue {
			return nil, fmt.Errorf("got mixed key types")
		}
		inObj, ok := in.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected list, got %T", inObj)
		}
		obj := make([]interface{}, 0, fieldSet.Children.Size()+fieldSet.Members.Size())
		fieldSet.Members.Iterate(func(pe fieldpath.PathElement) {
			obj = append(obj, inObj[*pe.Index])
		})
		fieldSet.Children.Iterate(func(pe fieldpath.PathElement) {
			child, err := extractFields(inObj[*pe.Index], fieldSet.WithPrefix(pe))
			if err == nil {
				obj = append(obj, child)
			}
		})
		out = obj
	case isValue:
		// Is this reachable?
		if isFieldName || isKey || isIndex {
			return nil, fmt.Errorf("got mixed key types")
		}
		out = in
	}

	return
}
