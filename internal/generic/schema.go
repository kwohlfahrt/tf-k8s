package generic

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

func OpenApiToTfSchema(ctx context.Context, customType types.KubernetesObjectType, isDatasSource bool) (schema.Attribute, error) {
	// TODO: Add validator for metadata fields
	return customType.SchemaType(ctx, types.SchemaOptions{IsDataSource: isDatasSource}, false)
}
