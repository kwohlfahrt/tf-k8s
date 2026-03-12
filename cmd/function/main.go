package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider/fn"
)

var version string = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "kwohlfahrt.github.io/tf-k8s/k8s-function",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), fn.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
