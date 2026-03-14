package generic

import (
	"bytes"
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/tf-k8s/internal/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type PlanOrState interface {
	GetAttribute(ctx context.Context, path path.Path, target interface{}) diag.Diagnostics
}

type Schema interface {
	Type() attr.Type
}

type ObjectMeta struct {
	Name      string
	Namespace string
}

func StateToObjectMeta(ctx context.Context, state PlanOrState, typeInfo TypeInfo, meta *ObjectMeta) diag.Diagnostics {
	var diags diag.Diagnostics

	var manifest types.KubernetesObjectValue
	diags.Append(state.GetAttribute(ctx, path.Root("manifest"), &manifest)...)
	if diags.HasError() {
		return diags
	}

	attrs := manifest.Attributes()
	metaAttrs := attrs["metadata"].(types.KubernetesObjectValue).Attributes()
	meta.Name = metaAttrs["name"].(basetypes.StringValue).ValueString()
	if typeInfo.Namespaced {
		meta.Namespace = metaAttrs["namespace"].(basetypes.StringValue).ValueString()
	}

	return diags
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
