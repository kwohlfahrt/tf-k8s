package generic

import (
	"bytes"
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type PlanOrState interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

func StateToValue(ctx context.Context, state PlanOrState, typeInfo TypeInfo) (types.KubernetesValue, diag.Diagnostics) {
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
	return objectValue, diags
}

var defaultFields fieldpath.Set

func init() {
	apiVersion := "apiVersion"
	defaultFields.Insert(fieldpath.Path{fieldpath.PathElement{FieldName: &apiVersion}})
	kind := "kind"
	defaultFields.Insert(fieldpath.Path{fieldpath.PathElement{FieldName: &kind}})
	metadata := "metadata"
	name := "name"
	defaultFields.Insert(fieldpath.Path{
		fieldpath.PathElement{FieldName: &metadata},
		fieldpath.PathElement{FieldName: &name},
	})
	namespace := "namespace"
	defaultFields.Insert(fieldpath.Path{
		fieldpath.PathElement{FieldName: &metadata},
		fieldpath.PathElement{FieldName: &namespace},
	})
}

func GetManagedFieldSet(in *unstructured.Unstructured, fieldManager string) (*fieldpath.Set, diag.Diagnostics) {
	var entry *v1.ManagedFieldsEntry
	for _, maybeEntry := range in.GetManagedFields() {
		if maybeEntry.Manager == fieldManager && maybeEntry.Operation == v1.ManagedFieldsOperationApply {
			entry = &maybeEntry
		}
	}

	fieldSet := &fieldpath.Set{}
	if entry != nil {
		err := fieldSet.FromJSON(bytes.NewReader(entry.FieldsV1.Raw))
		if err != nil {
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Unable to parse managed fields", err.Error())}
		}
	}
	fieldSet = fieldSet.Union(&defaultFields)
	return fieldSet, nil
}
