package provider

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

func TestAccCertificateResource(t *testing.T) {
	kubeconfig, err := os.ReadFile("../../examples/k8scrd/kubeconfig.yaml")
	if err != nil {
		t.Fatal(err)
	}

	cc, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := cc.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	k, err := dynamic.NewForConfig(cfg)
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
			"k8scrd_certificate": map[string]interface{}{
				"bar": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "bar",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"dns_names":   []string{"bar.example.com"},
						"secret_name": "bar",
						"issuer_ref": map[string]interface{}{
							"group": "cert-manager.io",
							"kind":  "ClusterIssuer",
							"name":  "test",
						},
					},
				},
			},
		},
	}

	configJson, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8scrd": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: string(configJson),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue(
							"k8scrd_certificate.bar",
							tfjsonpath.New("spec").AtMapKey("dns_names").AtSliceIndex(0),
							knownvalue.StringExact("bar.example.com"),
						),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"k8scrd_certificate.bar",
						tfjsonpath.New("spec").AtMapKey("dns_names").AtSliceIndex(0),
						knownvalue.StringExact("bar.example.com"),
					),
				},
				Check: func(s *terraform.State) error {
					return assertExists(k, "default", "bar", true)
				},
			},
		},
		CheckDestroy: func(s *terraform.State) error {
			for _, resource := range s.RootModule().Resources {
				if resource.Type != "k8scrd_certificate" {
					continue
				}
				components := strings.SplitN(resource.Primary.ID, "/", 2)
				assertExists(k, components[0], components[1], false)
			}
			return nil
		},
	})
}
