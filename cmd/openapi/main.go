package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/openapi3"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type openapiConfig struct {
	ApiGroups []string `json:"apiGroups"`
	Defaults  map[string]struct {
		Property string      `json:"property"`
		Value    interface{} `json:"value"`
	} `json:"defaults"`
}

var kubeconfig *string = flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Kubernetes config file path")

func getPath(gv runtimeschema.GroupVersion, resource metav1.APIResource) string {
	segments := make([]string, 1, 8)
	if gv.Group == "" {
		segments = append(segments, "api")
	} else {
		segments = append(segments, "apis", gv.Group)
	}
	segments = append(segments, gv.Version)

	if resource.Namespaced {
		segments = append(segments, "namespaces", "{namespace}")
	}
	segments = append(segments, resource.Name, "{name}")

	return strings.Join(segments, "/")
}

func getSchema(openapi *spec3.OpenAPI, gv runtimeschema.GroupVersion, resource metav1.APIResource) (*spec.Schema, error) {
	path := getPath(gv, resource)
	response := openapi.Paths.Paths[path].PathProps.Get.Responses.StatusCodeResponses[200]
	schemaRef := response.ResponseProps.Content["application/json"].MediaTypeProps.Schema
	maybeSchema, _, err := schemaRef.Ref.GetPointer().Get(openapi)
	if err != nil {
		return nil, err
	}
	schema, ok := maybeSchema.(*spec.Schema)
	if !ok {
		return nil, fmt.Errorf("expected schema, got %T", maybeSchema)
	}
	return schema, nil
}

func main() {
	flag.Parse()

	dataFile, err := os.Create("typeInfos.bin")
	if err != nil {
		log.Fatal((err.Error()))
	}
	gob.Register(types.KubernetesObjectType{})
	gob.Register(types.KubernetesListType{})
	gob.Register(types.KubernetesMapType{})
	gob.Register(types.KubernetesUnionType{})
	gob.Register(types.KubernetesUnknownType{})
	gob.Register(basetypes.BoolType{})
	gob.Register(basetypes.Int64Type{})
	gob.Register(basetypes.Float64Type{})
	gob.Register(basetypes.NumberType{})
	gob.Register(basetypes.StringType{})
	enc := gob.NewEncoder(dataFile)

	configFile, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err.Error())
	}
	configReader := utilyaml.NewYAMLToJSONDecoder(bufio.NewReader(configFile))
	var config openapiConfig
	if err = configReader.Decode(&config); err != nil {
		log.Fatal(err)
	}

	groups := make(map[string]bool, flag.NArg())
	for _, arg := range config.ApiGroups {
		groups[arg] = true
	}

	kubeconfigBytes, err := os.ReadFile(*kubeconfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	discoveryClient, err := provider.MakeDiscoveryClient(kubeconfigBytes)
	if err != nil {
		log.Fatal(err.Error())
	}
	root := openapi3.NewRoot(discoveryClient.OpenAPIV3())

	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		log.Fatal(err)
	}

	for _, resourceList := range resourceLists {
		gv, err := runtimeschema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			log.Fatal(err.Error())
		}
		if !groups[gv.Group] {
			continue
		}

		openApiSpec, err := root.GVSpec(gv)
		if err != nil {
			log.Fatal(err.Error())
		}

		for _, resource := range resourceList.APIResources {
			if strings.Contains(resource.Name, "/") {
				continue // Skip subresources
			}
			if slices.Index(resource.Verbs, "get") == -1 {
				// We generate the schema from get endpoint, so skip non-gettable (for now)
				continue
			}
			schema, err := getSchema(openApiSpec, gv, resource)
			if err != nil {
				log.Fatal(err.Error())
			}

			typ, err := types.OpenApiToTfType(openApiSpec, *schema, []string{})
			if err != nil {
				log.Fatal(err.Error())
			}
			objectTyp, ok := typ.(types.KubernetesObjectType)
			if !ok {
				log.Fatalf("expected KubernetesObjectType, got %T", objectTyp)
			}
			if _, found := objectTyp.AttrTypes["api_version"]; !found {
				continue
			}
			delete(objectTyp.AttrTypes, "api_version")
			if _, found := objectTyp.AttrTypes["kind"]; !found {
				continue
			}
			delete(objectTyp.AttrTypes, "kind")

			metaTyp, ok := objectTyp.AttrTypes["metadata"].(types.KubernetesObjectType)
			if !ok {
				log.Fatalf("expected KubernetesObjectType at .metadata, got %T", objectTyp)
			}
			delete(metaTyp.AttrTypes, "managed_fields")
			delete(metaTyp.AttrTypes, "generation")
			delete(metaTyp.AttrTypes, "resource_version")
			if !resource.Namespaced {
				delete(metaTyp.AttrTypes, "namespace")
			}

			info := generic.TypeInfo{
				Group:      gv.Group,
				Version:    gv.Version,
				Kind:       resource.Kind,
				Resource:   resource.Name,
				Namespaced: resource.Namespaced,
				Schema:     objectTyp,
			}

			err = enc.Encode(info)
			if err != nil {
				log.Fatal(err.Error())
			}
		}
	}
}
