package generic

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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

func (t TypeInfo) Codegen(builder *strings.Builder) {
	builder.WriteString("{")
	builder.WriteString(fmt.Sprintf("Group: %s, ", strconv.Quote(t.Group)))
	builder.WriteString(fmt.Sprintf("Resource: %s, ", strconv.Quote(t.Resource)))
	builder.WriteString(fmt.Sprintf("Kind: %s, ", strconv.Quote(t.Kind)))
	builder.WriteString(fmt.Sprintf("Version: %s, ", strconv.Quote(t.Version)))
	builder.WriteString("Schema: ")
	t.Schema.Codegen(builder)
	builder.WriteString("}")
}

func OpenApiToTfSchema(ctx context.Context, customType types.KubernetesType, datasource bool) (*schema.Schema, error) {
	specAttribute, err := customType.SchemaType(ctx, datasource, !datasource)
	if err != nil {
		return nil, err
	}

	metaAttribute, err := types.MetadataType.SchemaType(ctx, false, true)
	if err != nil {
		return nil, err
	}

	// TODO: Handle status field
	attributes := make(map[string]schema.Attribute, 2)
	if !datasource {
		attributes["id"] = schema.StringAttribute{Computed: true}
	}
	attributes["metadata"] = metaAttribute
	attributes["spec"] = specAttribute

	return &schema.Schema{Attributes: attributes}, nil
}
