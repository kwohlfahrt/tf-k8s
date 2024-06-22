package crd_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider/crd"
)

func TestAccDataSource(t *testing.T) {
	kubeconfig, err := os.ReadFile("../../../examples/k8scrd/kubeconfig.yaml")
	if err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"k8scrd": map[string]interface{}{
				"kubeconfig": string(kubeconfig),
			},
		},
		"data": map[string]interface{}{
			"k8scrd_foo_example_com_v1": map[string]interface{}{
				"foo": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "default",
					},
				},
			},
		},
	}

	configJson, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	providerFactory, err := crd.New("test")
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8scrd": providerserver.NewProtocol6WithError(providerFactory()),
		},
		Steps: []resource.TestStep{
			{
				Config: string(configJson),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.k8scrd_foo_example_com_v1.foo",
						tfjsonpath.New("spec").AtMapKey("foo"),
						knownvalue.StringExact("foo"),
					),
				},
			},
		},
	})
}

func TestAccDataSourceBuiltin(t *testing.T) {
	kubeconfig, err := os.ReadFile("../../../examples/k8scrd/kubeconfig.yaml")
	if err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"k8scrd": map[string]interface{}{
				"kubeconfig": string(kubeconfig),
			},
		},
		"data": map[string]interface{}{
			"k8scrd_deployment_apps_v1": map[string]interface{}{
				"foo": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "default",
					},
				},
			},
			"k8scrd_configmap_v1": map[string]interface{}{
				"foo": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "foo",
						"namespace": "default",
					},
				},
			},
		},
	}

	configJson, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	providerFactory, err := crd.New("test")
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8scrd": providerserver.NewProtocol6WithError(providerFactory()),
		},
		Steps: []resource.TestStep{
			{
				Config: string(configJson),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.k8scrd_deployment_apps_v1.foo",
						tfjsonpath.New("spec").AtMapKey("replicas"),
						knownvalue.Int64Exact(0),
					),
					statecheck.ExpectKnownValue(
						"data.k8scrd_configmap_v1.foo",
						tfjsonpath.New("data").AtMapKey("foo.txt"),
						knownvalue.StringExact("hello, world!\n"),
					),
				},
			},
		},
	})
}
