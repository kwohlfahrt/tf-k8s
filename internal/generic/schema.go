package generic

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

type TypeInfo struct {
	Group    string
	Resource string
	Kind     string
	Version  string
	Schema   types.KubernetesObjectType
}

func (t TypeInfo) GroupVersionResource() runtimeschema.GroupVersionResource {
	return runtimeschema.GroupVersionResource{
		Group:    t.Group,
		Resource: t.Resource,
		Version:  t.Version,
	}
}

func (t TypeInfo) Codegen(builder io.StringWriter) {
	builder.WriteString("{")
	builder.WriteString(fmt.Sprintf("Group: %s, ", strconv.Quote(t.Group)))
	builder.WriteString(fmt.Sprintf("Resource: %s, ", strconv.Quote(t.Resource)))
	builder.WriteString(fmt.Sprintf("Kind: %s, ", strconv.Quote(t.Kind)))
	builder.WriteString(fmt.Sprintf("Version: %s, ", strconv.Quote(t.Version)))
	builder.WriteString("Schema: ")
	t.Schema.Codegen(builder)
	builder.WriteString("}")
}

func OpenApiToTfSchema(ctx context.Context, customType types.KubernetesObjectType) (*schema.Schema, error) {
	attributes, err := customType.SchemaAttributes(ctx, false)
	if err != nil {
		return nil, err
	}

	meta, ok := attributes["metadata"].(schema.SingleNestedAttribute)
	if !ok {
		return nil, fmt.Errorf("expected object attribute at metadata")
	}
	meta.Computed = false
	meta.Optional = false
	meta.Required = true
	for _, attrName := range []string{"name", "namespace"} {
		attr, ok := meta.Attributes[attrName].(schema.StringAttribute)
		if !ok {
			return nil, fmt.Errorf("expected string attribute at metadata.%s", attrName)
		}
		attr.Computed = false
		attr.Optional = false
		attr.Required = true
		meta.Attributes[attrName] = attr
	}
	attributes["metadata"] = meta

	status, ok := attributes["status"].(schema.SingleNestedAttribute)
	if ok {
		status.Computed = true
		attributes["status"] = status
	}

	return &schema.Schema{Attributes: attributes}, nil
}
