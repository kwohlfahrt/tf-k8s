package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider/crd"
)

var (
	provider string = "example"
	version  string = "dev"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: fmt.Sprintf("kwohlfahrt.github.io/tf-k8s/k8s-%s", provider),
		Debug:   debug,
	}
	providerFactory, err := crd.New(version)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = providerserver.Serve(context.Background(), providerFactory, opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
