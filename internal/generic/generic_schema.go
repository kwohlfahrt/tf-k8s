package generic

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func LoadCrd(bytes []byte, requiredVersion string) (map[string]interface{}, error) {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}

	_, _, err := decoder.Decode(bytes, nil, obj)
	if err != nil {
		return nil, err
	}

	versions, found, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "versions")
	if err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("field not found: spec.versions")
	}

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

		if versionName != requiredVersion {
			continue
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
		return schema, nil
	}

	return nil, nil
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

	customType, err := FromOpenApi(specProperties, []string{"spec"})
	if err != nil {
		return nil, err
	}

	specAttributes, err := customType.SchemaAttributes(ctx, datasource)
	if err != nil {
		return nil, err
	}

	// TODO: Handle status field
	attributes := make(map[string]schema.Attribute, 2)
	if !datasource {
		attributes["id"] = schema.StringAttribute{Computed: true}
	}
	attributes["metadata"] = schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"name":      schema.StringAttribute{Required: true},
			"namespace": schema.StringAttribute{Required: true},
		},
	}
	attributes["spec"] = schema.SingleNestedAttribute{
		Required:   !datasource,
		Computed:   datasource,
		Attributes: specAttributes,
		CustomType: *customType,
	}

	return &schema.Schema{Attributes: attributes}, nil
}
