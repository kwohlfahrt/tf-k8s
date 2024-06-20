package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	flag "github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi3"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

var kubeconfig *string = flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Kubernetes config file path")

func getSchema(openapi *spec3.OpenAPI, gv runtimeschema.GroupVersion, resource metav1.APIResource) (*spec.Schema, error) {
	path := fmt.Sprintf("/apis/%s/%s/namespaces/{namespace}/%s/{name}", gv.Group, gv.Version, resource.Name)
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

	file, err := os.Create("crd.go")
	if err != nil {
		log.Fatal(err.Error())
	}

	groups := make(map[string]bool, flag.NArg())
	for _, arg := range flag.Args() {
		groups[arg] = true
	}

	config, err := os.ReadFile(*kubeconfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	discoveryClient, err := provider.MakeDiscoveryClient(config)
	if err != nil {
		log.Fatal(err.Error())
	}
	root := openapi3.NewRoot(discoveryClient.OpenAPIV3())

	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		log.Fatal(err)
	}

	file.WriteString("package crd\n\n")
	file.WriteString("import (\n")
	file.WriteString("\t\"github.com/hashicorp/terraform-plugin-framework/attr\"\n")
	file.WriteString("\t\"github.com/hashicorp/terraform-plugin-framework/types/basetypes\"\n")
	file.WriteString("\t\"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types\"\n")
	file.WriteString("\t\"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic\"\n")
	file.WriteString(")\n\nvar TypeInfos = []generic.TypeInfo{")
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
			schema, err := getSchema(openApiSpec, gv, resource)
			if err != nil {
				log.Fatalf(err.Error())
			}

			spec, found := schema.Properties["spec"]
			if !found {
				continue
			}

			typ, err := types.OpenApiToTfType(openApiSpec, spec, []string{})
			if err != nil {
				log.Fatal(err.Error())
			}
			objectTyp, ok := typ.(types.KubernetesObjectType)
			if !ok {
				log.Fatalf("expected KubernetesObjectType, got %T", objectTyp)
			}

			info := generic.TypeInfo{
				Group:    gv.Group,
				Version:  gv.Version,
				Kind:     resource.Kind,
				Resource: resource.Name,
				Schema:   objectTyp,
			}
			info.Codegen(file)
			file.WriteString(", ")
		}
	}

	file.WriteString("}")
}
