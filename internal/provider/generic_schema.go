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

func openApiToTfSchema(openapi map[string]interface{}) (*schema.Schema, error) {
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

	spec, err := propertiesToAttributes([]string{"spec"}, specProperties)
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
	attributes["spec"] = schema.SingleNestedAttribute{Required: true, Attributes: spec}

	return &schema.Schema{Attributes: attributes}, nil
}

func openApiToTfAttribute(path []string, openapi map[string]interface{}) (schema.Attribute, error) {
	switch ty := openapi["type"]; ty {
	case "object":
		attributes, err := propertiesToAttributes(path, openapi)
		if err != nil {
			return nil, err
		}
		return &schema.SingleNestedAttribute{
			Required:   true,
			Attributes: attributes,
		}, nil
	case "array":
		items, ok := openapi["items"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("object %s has no items", strings.Join(path, ""))
		}

		attribute, err := openApiToTfAttribute(append(path, "[*]"), items)
		if err != nil {
			return nil, err
		}

		switch attr := attribute.(type) {
		case schema.SingleNestedAttribute:
			return &schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes:    attr.Attributes,
					CustomType:    attr.CustomType,
					Validators:    attr.Validators,
					PlanModifiers: attr.PlanModifiers,
				},
			}, nil
		default:
			return &schema.ListAttribute{Required: true, ElementType: attr.GetType()}, nil
		}
	case "string":
		return &schema.StringAttribute{Required: true}, nil
	case "integer":
		return &schema.Int64Attribute{Required: true}, nil
	case "boolean":
		return &schema.BoolAttribute{Required: true}, nil
	case "":
		return nil, fmt.Errorf("schema item has no type at %s", strings.Join(path, ""))
	default:
		return nil, fmt.Errorf("unrecognized type at %s: %s", strings.Join(path, ""), ty)
	}
}

func propertiesToAttributes(path []string, openapi map[string]interface{}) (map[string]schema.Attribute, error) {
	rawProperties, ok := openapi["properties"]
	if !ok {
		return nil, nil
	}

	properties := rawProperties.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("object has no properties at %s", strings.Join(path, ""))
	}

	attributes := make(map[string]schema.Attribute, len(properties))
	for k, v := range properties {
		attrPath := append(path, fmt.Sprintf(".%s", k))
		property, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map of properties at %s", strings.Join(attrPath, ""))
		}

		attribute, err := openApiToTfAttribute(attrPath, property)
		if err != nil {
			return nil, err
		}
		attributes[strcase.SnakeCase(k)] = attribute
	}
	return attributes, nil
}
