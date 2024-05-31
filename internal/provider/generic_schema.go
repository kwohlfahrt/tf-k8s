package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	strcase "github.com/stoewer/go-strcase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func loadCrd(bytes []byte, requiredVersion string) (map[string]interface{}, error) {
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

func openApiToTfSchema(openapi map[string]interface{}, datasource bool) (*schema.Schema, error) {
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

	spec, err := propertiesToAttributes([]string{"spec"}, specProperties, datasource)
	if err != nil {
		return nil, err
	}

	// TODO: Handle status field
	attributes := make(map[string]schema.Attribute, 2)
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
		Attributes: spec,
	}

	return &schema.Schema{Attributes: attributes}, nil
}

func openApiToTfAttribute(path []string, openapi map[string]interface{}, datasource bool) (schema.Attribute, error) {
	switch ty := openapi["type"]; ty {
	case "object":
		if rawItems, isMap := openapi["additionalProperties"]; isMap {
			items, ok := rawItems.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected additionalProperties object at %s", strings.Join(path, ""))
			}
			attribute, err := openApiToTfAttribute(append(path, "[*]"), items, datasource)
			if err != nil {
				return nil, err
			}
			switch attr := attribute.(type) {
			case schema.SingleNestedAttribute:
				return schema.MapNestedAttribute{
					Required:     !datasource,
					Computed:     datasource,
					NestedObject: objectToNestedObject(attr),
				}, nil
			default:
				return schema.MapAttribute{
					Required:    !datasource,
					Computed:    datasource,
					ElementType: attr.GetType(),
				}, nil
			}
		} else {
			attributes, err := propertiesToAttributes(path, openapi, datasource)
			if err != nil {
				return nil, err
			}
			return schema.SingleNestedAttribute{
				Required:   !datasource,
				Computed:   datasource,
				Attributes: attributes,
			}, nil
		}
	case "array":
		items, ok := openapi["items"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("object %s has no items", strings.Join(path, ""))
		}

		attribute, err := openApiToTfAttribute(append(path, "[*]"), items, datasource)
		if err != nil {
			return nil, err
		}

		switch attr := attribute.(type) {
		case schema.SingleNestedAttribute:
			return schema.ListNestedAttribute{
				Required:     !datasource,
				Computed:     datasource,
				NestedObject: objectToNestedObject(attr),
			}, nil
		default:
			return schema.ListAttribute{
				Required:    !datasource,
				Computed:    datasource,
				ElementType: attr.GetType(),
			}, nil
		}
	case "string":
		return schema.StringAttribute{
			Required: !datasource,
			Computed: datasource,
		}, nil
	case "integer":
		return schema.Int64Attribute{
			Required: !datasource,
			Computed: datasource,
		}, nil
	case "boolean":
		return schema.BoolAttribute{
			Required: !datasource,
			Computed: datasource,
		}, nil
	case "":
		return nil, fmt.Errorf("schema item has no type at %s", strings.Join(path, ""))
	default:
		return nil, fmt.Errorf("unrecognized type at %s: %s", strings.Join(path, ""), ty)
	}
}

func propertiesToAttributes(path []string, openapi map[string]interface{}, datasource bool) (map[string]schema.Attribute, error) {
	properties, ok := openapi["properties"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map of properties at %s", strings.Join(path, ""))
	}

	var required map[string]bool
	rawRequired, found := openapi["required"]
	if found {
		requiredKeys, ok := rawRequired.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected list of required attributes at %s", strings.Join(path, ""))
		}
		required = make(map[string]bool, len(requiredKeys))
		for i, k := range requiredKeys {
			name, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("expected attribute name at %s[%d]", strings.Join(path, ""), i)
			}
			required[name] = true
		}
	}

	attributes := make(map[string]schema.Attribute, len(properties))
	for k, v := range properties {
		attrPath := append(path, fmt.Sprintf(".%s", k))
		property, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object at %s", strings.Join(attrPath, ""))
		}

		attribute, err := openApiToTfAttribute(attrPath, property, datasource)
		if err != nil {
			return nil, err
		}
		attributes[strcase.SnakeCase(k)] = attribute
	}
	return attributes, nil
}
