package crd_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider/crd"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAccResource(t *testing.T) {
	kubeconfig, err := os.ReadFile("../../../examples/k8scrd/kubeconfig.yaml")
	if err != nil {
		t.Fatal(err)
	}

	k, err := provider.MakeDynamicClient(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"k8scrd": map[string]interface{}{
				"kubeconfig": string(kubeconfig),
			},
		},
		"resource": map[string]interface{}{
			"k8scrd_foo_example_com_v1": map[string]interface{}{
				"bar": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "bar",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			"k8scrd_bar_example_com_v1": map[string]interface{}{
				"bar": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "bar",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"bar": "bar",
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
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(
							"k8scrd_foo_example_com_v1.bar",
							tfjsonpath.New("spec").AtMapKey("foo"),
							knownvalue.StringExact("bar"),
						),
						plancheck.ExpectKnownValue(
							"k8scrd_bar_example_com_v1.bar",
							tfjsonpath.New("spec").AtMapKey("bar"),
							knownvalue.StringExact("bar"),
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"k8scrd_foo_example_com_v1.bar",
						tfjsonpath.New("spec").AtMapKey("foo"),
						knownvalue.StringExact("bar"),
					),
					statecheck.ExpectKnownValue(
						"k8scrd_bar_example_com_v1.bar",
						tfjsonpath.New("spec").AtMapKey("bar"),
						knownvalue.StringExact("bar"),
					),
				},
				Check: resource.ComposeTestCheckFunc(
					checkExists(
						k,
						schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "foos"},
						types.ObjectMeta{Namespace: "default", Name: "bar"},
						true,
					),
					checkExists(
						k,
						schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "bars"},
						types.ObjectMeta{Namespace: "default", Name: "bar"},
						true,
					),
				),
			},
		},
		CheckDestroy: resource.ComposeTestCheckFunc(
			checkExists(
				k,
				schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "foos"},
				types.ObjectMeta{Namespace: "default", Name: "bar"},
				false,
			),
			checkExists(
				k,
				schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "bars"},
				types.ObjectMeta{Namespace: "default", Name: "bar"},
				false,
			),
		),
	})
}
