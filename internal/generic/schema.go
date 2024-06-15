package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

type TypeInfo struct {
	Group    string
	Resource string
	Kind     string
	Version  string
	Schema   map[string]interface{}
}

func (t TypeInfo) GroupVersionResource() runtimeschema.GroupVersionResource {
	return runtimeschema.GroupVersionResource{
		Group:    t.Group,
		Resource: t.Resource,
		Version:  t.Version,
	}
}

func LoadCrd(bytes []byte) (map[string]TypeInfo, error) {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}

	_, _, err := decoder.Decode(bytes, nil, obj)
	if err != nil {
		return nil, err
	}

	group, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "group")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("field not found: spec.group")
	}

	kind, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "names", "kind")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("field not found: spec.names.kind")
	}

	resource, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "names", "plural")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("field not found: spec.names.resource")
	}

	versions, found, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "versions")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("field not found: spec.versions")
	}

	typeInfos := make(map[string]TypeInfo, len(versions))
	for _, version := range versions {
		versionObj, ok := version.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object, found %t", version)
		}

		versionName, found, err := unstructured.NestedString(versionObj, "name")
		if err != nil {
			return nil, err
		} else if !found {
			return nil, fmt.Errorf("field not found: spec.versions[*].name")
		}

		schemaField, found, err := unstructured.NestedFieldNoCopy(versionObj, "schema", "openAPIV3Schema")
		if err != nil {
			return nil, err
		} else if !found {
			return nil, fmt.Errorf("field not found: spec.versions[*].name")
		}

		schema, ok := schemaField.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object, found %t", schemaField)
		}
		typeInfos[versionName] = TypeInfo{
			Group:    group,
			Version:  versionName,
			Resource: resource,
			Kind:     kind,
			Schema:   schema,
		}
	}

	return typeInfos, nil
}

func OpenApiToTfSchema(ctx context.Context, openapi map[string]interface{}, datasource bool) (*schema.Schema, error) {
	if ty := openapi["type"]; ty != "object" {
		return nil, fmt.Errorf("expected object, got type: %s", ty)
	}

	properties, ok := openapi["properties"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("object has no properties")
	}

	specProperties, ok := properties["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("object spec has invalid properties")
	}

	customType, err := types.ObjectFromOpenApi(specProperties, []string{"spec"})
	if err != nil {
		return nil, err
	}

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
