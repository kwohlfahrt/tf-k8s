package generic

import (
	"bytes"
	"context"
	"fmt"
	"maps"
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
func Extract(in *unstructured.Unstructured, fieldManager string) (*unstructured.Unstructured, diag.Diagnostics) {
	var entry *v1.ManagedFieldsEntry
	for _, maybeEntry := range in.GetManagedFields() {
		if maybeEntry.Manager == fieldManager && maybeEntry.Operation == v1.ManagedFieldsOperationApply {
			entry = &maybeEntry
		}
	}
	object := map[string]interface{}{}
	if entry != nil {
		fieldSet := &fieldpath.Set{}
		err := fieldSet.FromJSON(bytes.NewReader(entry.FieldsV1.Raw))
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Unable to parse managed fields", err.Error())}
		}

		content, diags := extractFields(in.UnstructuredContent(), path.Empty(), fieldSet.Leaves())
		if diags.HasError() {
			return nil, diags
		}

		object = content.(map[string]interface{})
	}
	object["apiVersion"] = in.GetAPIVersion()
	object["kind"] = in.GetKind()

	var metadata map[string]interface{}
	if object["metadata"] != nil {
		metadata = object["metadata"].(map[string]interface{})
	} else {
		metadata = make(map[string]interface{}, 2)
		object["metadata"] = metadata
	}
	metadata["name"] = in.GetName()
	metadata["namespace"] = in.GetNamespace()

	return &unstructured.Unstructured{Object: object}, nil
}

func extractFields(in interface{}, path path.Path, fieldSet *fieldpath.Set) (out interface{}, diags diag.Diagnostics) {
	fieldTypes := make(map[string]bool, 4)

	checkType := func(pe fieldpath.PathElement) {
		if pe.FieldName != nil {
			fieldTypes["FieldName"] = true
		}
		if pe.Key != nil {
			fieldTypes["Key"] = true
		}
		if pe.Index != nil {
			fieldTypes["Index"] = true
		}
		if pe.Value != nil {
			fieldTypes["Value"] = true
		}
	}

	fieldSet.Members.Iterate(checkType)
	fieldSet.Children.Iterate(checkType)

	if len(fieldTypes) > 1 {
		return nil, diag.Diagnostics{diag.NewAttributeErrorDiagnostic(
			path, "Got mixed index types", fmt.Sprintf("%v", maps.Keys(fieldTypes)),
		)}
	}

	switch {
	case fieldTypes["FieldName"]:
		inObj, ok := in.(map[string]interface{})
		if !ok {
			return nil, diag.Diagnostics{diag.NewAttributeErrorDiagnostic(
				path, "Expected map type", fmt.Sprintf("got: %T", in),
			)}
		}

		obj := make(map[string]interface{}, fieldSet.Children.Size()+fieldSet.Members.Size())
		for k, v := range inObj {
			p := fieldpath.PathElement{FieldName: &k}
			if fieldSet.Members.Has(p) {
				obj[k] = v
			} else if childSet, found := fieldSet.Children.Get(p); found {
				// Can't distinguish between map keys and attributes
				child, err := extractFields(v, path.AtMapKey(k), childSet)
				if err == nil {
					obj[k] = child
				}
			}
		}
		out = obj
	case fieldTypes["Key"]:
		inObj, ok := in.([]interface{})
		if !ok {
			return nil, diag.Diagnostics{diag.NewAttributeErrorDiagnostic(
				path, "Expected slice type", fmt.Sprintf("got: %T", in),
			)}
		}

		ke, diag := newKeyExtractor(fieldSet)
		if diag != nil {
			diags.AddAttributeError(path, diag.Summary(), diag.Detail())
			return nil, diags
		}
		obj := make([]interface{}, 0, fieldSet.Children.Size()+fieldSet.Members.Size())

		for i, v := range inObj {
			if v == nil {
				continue
			}
			p := ke.extractKey(v.(map[string]interface{}))
			if fieldSet.Members.Has(p) {
				obj = append(obj, v)
			} else if childSet, found := fieldSet.Children.Get(p); found {
				child, err := extractFields(v, path.AtListIndex(i), childSet)
				if err == nil {
					obj = append(obj, child)
				}
			}
		}

		out = obj
	case fieldTypes["Index"]:
		inObj, ok := in.([]interface{})
		if !ok {
			return nil, diag.Diagnostics{diag.NewAttributeErrorDiagnostic(
				path, "Expected slice type", fmt.Sprintf("got: %T", in),
			)}
		}
		obj := make([]interface{}, 0, fieldSet.Children.Size()+fieldSet.Members.Size())
		for i, v := range inObj {
			p := fieldpath.PathElement{Index: &i}
			if fieldSet.Members.Has(p) {
				obj = append(obj, v)
			} else if childSet, found := fieldSet.Children.Get(p); found {
				child, err := extractFields(v, path.AtListIndex(i), childSet)
				if err == nil {
					obj = append(obj, child)
				}
			}
		}
		out = obj
	case fieldTypes["Value"]:
		// Is this reachable?
		out = in
	}

	return
}

type keyExtractor struct {
	keyFields []string
}

func newKeyExtractor(fs *fieldpath.Set) (*keyExtractor, diag.Diagnostic) {
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
		return nil, diag.NewErrorDiagnostic("Mismatched key fields", "")
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
