package generic

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	strcase "github.com/stoewer/go-strcase"
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

func OpenApiToTfSchema(openapi map[string]interface{}, datasource bool) (*schema.Schema, error) {
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

	spec, err := propertiesToAttributes(datasource, []string{"spec."}, specProperties)
	if err != nil {
		return nil, err
	}
	customType, err := FromOpenApi(specProperties, []string{"spec."})
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
		Attributes: spec,
		CustomType: customType,
	}

	return &schema.Schema{Attributes: attributes}, nil
}

type config struct {
	datasource bool
	required   bool
}

func openApiToTfAttribute(config config, path []string, openapi map[string]interface{}) (schema.Attribute, error) {
	computed := config.datasource
	optional := !config.datasource && !config.required
	required := !config.datasource && config.required

	switch ty := openapi["type"]; ty {
	case "object":
		if rawItems, isMap := openapi["additionalProperties"]; isMap {
			items, ok := rawItems.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected additionalProperties object at %s", strings.Join(path, ""))
			}
			attribute, err := openApiToTfAttribute(config, append(path, "[*]"), items)
			if err != nil {
				return nil, err
			}
			switch attr := attribute.(type) {
			case schema.SingleNestedAttribute:
				return schema.MapNestedAttribute{
					Required:     required,
					Optional:     optional,
					Computed:     computed,
					NestedObject: objectToNestedObject(attr),
				}, nil
			default:
				return schema.MapAttribute{
					Required:    required,
					Optional:    optional,
					Computed:    computed,
					ElementType: attr.GetType(),
				}, nil
			}
		} else {
			attributes, err := propertiesToAttributes(config.datasource, path, openapi)
			if err != nil {
				return nil, err
			}
			customType, err := FromOpenApi(openapi, path)
			if err != nil {
				return nil, err
			}
			return schema.SingleNestedAttribute{
				Required:   required,
				Optional:   optional,
				Computed:   computed,
				Attributes: attributes,
				CustomType: customType,
			}, nil
		}
	case "array":
		items, ok := openapi["items"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("object %s has no items", strings.Join(path, ""))
		}

		attribute, err := openApiToTfAttribute(config, append(path, "[*]"), items)
		if err != nil {
			return nil, err
		}

		switch attr := attribute.(type) {
		case schema.SingleNestedAttribute:
			return schema.ListNestedAttribute{
				Required:     required,
				Optional:     optional,
				Computed:     computed,
				NestedObject: objectToNestedObject(attr),
			}, nil
		default:
			return schema.ListAttribute{
				Required:    required,
				Optional:    optional,
				Computed:    computed,
				ElementType: attr.GetType(),
			}, nil
		}
	case "string":
		return schema.StringAttribute{
			Required: required,
			Optional: optional,
			Computed: computed,
		}, nil
	case "integer":
		return schema.Int64Attribute{
			Required: required,
			Optional: optional,
			Computed: computed,
		}, nil
	case "boolean":
		return schema.BoolAttribute{
			Required: required,
			Optional: optional,
			Computed: computed,
		}, nil
	case "":
		return nil, fmt.Errorf("schema item has no type at %s", strings.Join(path, ""))
	default:
		return nil, fmt.Errorf("unrecognized type at %s: %s", strings.Join(path, ""), ty)
	}
}

func propertiesToAttributes(datasource bool, path []string, openapi map[string]interface{}) (map[string]schema.Attribute, error) {
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
	fieldNames := make(map[string]string, len(properties))
	for k, v := range properties {
		attrPath := append(path, fmt.Sprintf(".%s", k))
		property, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object at %s", strings.Join(attrPath, ""))
		}
		attrConfig := config{datasource: datasource, required: required[k]}

		attribute, err := openApiToTfAttribute(attrConfig, attrPath, property)
		if err != nil {
			return nil, err
		}
		fieldName := strcase.SnakeCase(k)
		attributes[fieldName] = attribute
		fieldNames[fieldName] = k
	}
	return attributes, nil
}
