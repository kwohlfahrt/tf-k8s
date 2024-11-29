package generic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type PlanOrState interface {
	GetAttribute(ctx context.Context, path path.Path, target interface{}) diag.Diagnostics
}

func StateToValue(ctx context.Context, state PlanOrState, typeInfo TypeInfo) (types.KubernetesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	var stateValue basetypes.ObjectValue
	diags.Append(state.GetAttribute(ctx, path.Root("manifest"), &stateValue)...)
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

type PrivateState interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

func GetImportFieldManager(ctx context.Context, private PrivateState, key string) (*string, diag.Diagnostics) {
	serialized, diags := private.GetKey(ctx, key)
	if diags.HasError() || serialized == nil {
		return nil, diags
	}

	fieldManagers := make([]string, 0)
	err := json.Unmarshal(serialized, &fieldManagers)
	if err != nil {
		diags.AddError("Error unmarshalling import field managers", fmt.Sprintf("expected valid JSON, got %v", serialized))
		return nil, diags
	}

	if len(fieldManagers) != 1 {
		diags.AddError(
			"Too many field managers for import",
			fmt.Sprintf("Only one field manager is supported for import, got %v", fieldManagers),
		)
	}

	return &fieldManagers[0], diags
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
