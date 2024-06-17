package main

import (
	"fmt"
	"log"
	"os"
	"strings"

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

	client := discoveryClient.OpenAPIV3()
	root := openapi3.NewRoot(client)
	_, err = root.GroupVersions()
	if err != nil {
		log.Fatal(err.Error())
	}

	gv := runtimeschema.GroupVersion{Group: "example.com", Version: "v1"}
	openApiSpec, err := root.GVSpec(gv)
	if err != nil {
		log.Fatal(err.Error())
	}

	schema := openApiSpec.Components.Schemas["com.example.v1.Foo"]
	spec, found := schema.Properties["spec"]
	if !found {
		log.Fatal(fmt.Errorf("no CRD spec found"))
	}

	typ, err := types.ObjectFromOpenApi(spec, []string{})
	if err != nil {
		log.Fatal(err.Error())
	}

	var builder strings.Builder
	typ.Codegen(&builder)
	fmt.Printf("%s\n", builder.String())
}
