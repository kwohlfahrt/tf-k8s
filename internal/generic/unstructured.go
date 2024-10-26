package generic

import (
	"bytes"
	"context"
	"fmt"
	"slices"

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
		for k, v := range inObj {
			p := fieldpath.PathElement{FieldName: &k}
			if fieldSet.Members.Has(p) {
				obj[k] = v
			} else if childSet, found := fieldSet.Children.Get(p); found {
				child, err := extractFields(v, childSet)
				if err == nil {
					obj[k] = child
				}
			}
		}
		out = obj
	case isKey:
		if isFieldName || isIndex || isValue {
			return nil, fmt.Errorf("got mixed key types")
		}
		inObj, ok := in.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected list, got %T", inObj)
		}

		ke, err := newKeyExtractor(fieldSet)
		if err != nil {
			return nil, err
		}
		obj := make([]interface{}, 0, fieldSet.Children.Size()+fieldSet.Members.Size())

		for _, v := range inObj {
			if v == nil {
				continue
			}
			p := ke.extractKey(v.(map[string]interface{}))
			if fieldSet.Members.Has(p) {
				obj = append(obj, v)
			} else if childSet, found := fieldSet.Children.Get(p); found {
				child, err := extractFields(v, childSet)
				if err == nil {
					obj = append(obj, child)
				}
			}
		}

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
		for i, v := range inObj {
			p := fieldpath.PathElement{Index: &i}
			if fieldSet.Members.Has(p) {
				obj = append(obj, v)
			} else if childSet, found := fieldSet.Children.Get(p); found {
				child, err := extractFields(v, childSet)
				if err == nil {
					obj = append(obj, child)
				}
			}
		}
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

type keyExtractor struct {
	keyFields []string
}

func newKeyExtractor(fs *fieldpath.Set) (*keyExtractor, error) {
	var keyFields []string

	mismatch := false
	getKeys := func(pe fieldpath.PathElement) {
		keys := make([]string, 0, len(*pe.Key))
		for _, k := range *pe.Key {
			keys = append(keys, k.Name)
		}
		if keyFields != nil {
			if !slices.Equal(keys, keyFields) {
				mismatch = true
			}
		} else {
			keyFields = keys
		}
	}
	fs.Members.Iterate(getKeys)
	fs.Children.Iterate(getKeys)

	if mismatch {
		return nil, fmt.Errorf("mismatched keys in field path")
	}
	return &keyExtractor{keyFields: keyFields}, nil
}

func (e keyExtractor) extractKey(obj map[string]interface{}) fieldpath.PathElement {
	fields := make(value.FieldList, 0, len(e.keyFields))
	for _, k := range e.keyFields {
		v := value.NewValueInterface(obj[k])
		fields = append(fields, value.Field{Name: k, Value: v})
	}
	return fieldpath.PathElement{Key: &fields}
}
