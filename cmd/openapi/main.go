package main

import (
	"fmt"
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
	config, err := os.ReadFile("./kubeconfig.yaml")
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

	var builder strings.Builder
	builder.WriteString("package internal\n\n")
	builder.WriteString("import (\n")
	builder.WriteString("\t\"github.com/hashicorp/terraform-plugin-framework/attr\"\n")
	builder.WriteString("\t\"github.com/hashicorp/terraform-plugin-framework/types/basetypes\"\n")
	builder.WriteString("\t\"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types\"\n")
	builder.WriteString("\t\"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic\"\n")
	builder.WriteString(")\n\nvar TypeInfos = []generic.TypeInfo{")
	for _, resourceList := range resourceLists {
		gv, err := runtimeschema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			log.Fatal(err.Error())
		}
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
			info.Codegen(&builder)
			builder.WriteString(", ")
		}
	}

	builder.WriteString("}")

	fmt.Println(builder.String())
}
