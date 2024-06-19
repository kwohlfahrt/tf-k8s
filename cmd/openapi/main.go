package main

import (
	"log"
	"os"
	"slices"
	"strings"

	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi3"
)

func main() {
	file, err := os.Create("crd.go")
	if err != nil {
		log.Fatal(err.Error())
	}

	// TODO: Use env var here
	config, err := os.ReadFile("../../../examples/k8scrd/kubeconfig.yaml")
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
		// TODO: Make this configurable
		if gv.Group != "example.com" {
			continue
		}
		groupComponents := strings.Split(gv.Group, ".")
		slices.Reverse(groupComponents)
		reverseGroup := strings.Join(groupComponents, ".")

		openApiSpec, err := root.GVSpec(gv)
		if err != nil {
			log.Fatal(err.Error())
		}

		for _, resource := range resourceList.APIResources {
			if strings.Contains(resource.Name, "/") {
				continue // Skip subresources
			}
			schemaName := strings.Join([]string{reverseGroup, gv.Version, resource.Kind}, ".")
			schema, found := openApiSpec.Components.Schemas[schemaName]
			if !found {
				log.Fatalf("schema not found for: %s", schemaName)
			}

			spec, found := schema.Properties["spec"]
			if !found {
				continue
			}

			typ, err := types.ObjectFromOpenApi(spec, []string{})
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
