package generic

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type TypeInfo struct {
	Group      string
	Resource   string
	Kind       string
	Version    string
	Namespaced bool
	Schema     types.KubernetesObjectType
}

func (t TypeInfo) GroupVersionResource() runtimeschema.GroupVersionResource {
	return runtimeschema.GroupVersionResource{
		Group:    t.Group,
		Resource: t.Resource,
		Version:  t.Version,
	}
}

func (t TypeInfo) Interface(client *dynamic.DynamicClient, namespace string) dynamic.ResourceInterface {
	namespaceable := client.Resource(t.GroupVersionResource())
	resource := namespaceable.(dynamic.ResourceInterface)
	if t.Namespaced {
		resource = namespaceable.Namespace(namespace)
	}
	return resource
}

func OpenApiToTfSchema(ctx context.Context, typeInfo TypeInfo, isDatasSource bool) (schema.Attribute, error) {
	attr, err := typeInfo.Schema.SchemaType(ctx, true)
	if err != nil {
		return nil, err
	}

	dynamicAttr := attr.(schema.DynamicAttribute)
	dynamicAttr.Validators = append(dynamicAttr.Validators, metadataValidator{isNamespaced: typeInfo.Namespaced})

	return dynamicAttr, err
}

type metadataValidator struct {
	isNamespaced bool
}

func (v metadataValidator) Description(ctx context.Context) string {
	return "Validate manifest metadata"
}

func (v metadataValidator) MarkdownDescription(ctx context.Context) string {
	fields := []string{"metadata.name"}
	if v.isNamespaced {
		fields = append(fields, "metadata.namespace")
	}
	return fmt.Sprintf("Validate manifest metadata contains fields: %s", strings.Join(fields, ", "))
}

func (v metadataValidator) ValidateDynamic(ctx context.Context, req validator.DynamicRequest, resp *validator.DynamicResponse) {
	underlying := req.ConfigValue.UnderlyingValue()
	value, ok := underlying.(basetypes.ObjectValue)
	if !ok {
		resp.Diagnostics.AddAttributeError(req.Path, "Unexpected value type", fmt.Sprintf("Expected object, got %T", underlying))
		return
	}
	metadataPath := req.Path.AtName("metadata")
	metadataValue, found := value.Attributes()["metadata"]
	if !found {
		resp.Diagnostics.AddAttributeError(metadataPath, "Missing metadata value", "Manifest does not contain metadata")
		return
	}

	metadata, ok := metadataValue.(types.KubernetesObjectValue)
	if !ok {
		resp.Diagnostics.AddAttributeError(metadataPath, "Unexpected value type", fmt.Sprintf("Expected object, got %T", underlying))
		return
	}

	attributes := metadata.Attributes()

	name := attributes["name"]
	if name.IsNull() {
		resp.Diagnostics.AddAttributeError(metadataPath.AtName("name"), "Missing attribute", "Manifest does not contain metadata.name")
	}
	namespace := attributes["namespace"]
	if v.isNamespaced && namespace.IsNull() {
		resp.Diagnostics.AddAttributeError(metadataPath.AtName("name"), "Missing attribute", "Manifest does not contain metadata.namespace")
	}
}
