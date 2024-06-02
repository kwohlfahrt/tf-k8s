package provider

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
)

func TestAccCertificateDataSource(t *testing.T) {
	kubeconfig, err := os.ReadFile("../../examples/k8scrd/kubeconfig.yaml")
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
			"k8scrd_certificate": map[string]interface{}{
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

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8scrd": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: string(configJson),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.k8scrd_certificate.foo",
						tfjsonpath.New("spec").AtMapKey("dns_names").AtSliceIndex(0),
						knownvalue.StringExact("foo.example.com"),
					),
				},
			},
		},
	})
}
