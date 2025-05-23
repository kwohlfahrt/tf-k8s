package crd_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider/crd"
)

func TestAccDataSource(t *testing.T) {
	kubeconfig, err := os.ReadFile(os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatal(err)
	}

	k, err := provider.MakeDynamicClient(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}

	providerFactory, err := crd.New("test")
	if err != nil {
		t.Fatal(err)
	}

	rawCheckSpec, err := os.ReadFile(fmt.Sprintf("fixtures/%s/data-test.json", os.Getenv("PROVIDER")))
	if err != nil {
		t.Fatal(err)
	}
	var checkSpec checkSpec
	err = json.Unmarshal(rawCheckSpec, &checkSpec)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := os.ReadFile(fmt.Sprintf("./fixtures/%s/data.tf", os.Getenv("PROVIDER")))
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8scrd": providerserver.NewProtocol6WithError(providerFactory()),
		},
		Steps: []resource.TestStep{
			{
				Config:            string(cfg),
				ConfigVariables:   config.Variables{"kubeconfig": config.StringVariable(string(kubeconfig))},
				Check:             makeChecks(k, checkSpec.Resources),
				ConfigStateChecks: makeStateChecks(checkSpec.Properties),
			},
		},
	})
}
