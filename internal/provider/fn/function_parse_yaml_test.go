package fn_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider/fn"
)

func TestParseYaml(t *testing.T) {
	yaml, err := os.ReadFile("./test-certificate.yaml")
	if err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"provider": map[string]interface{}{
			"k8sfn": map[string]interface{}{},
		},
		"locals": map[string]interface{}{
			"yaml": string(yaml),
		},
		"output": map[string]interface{}{
			"yaml": map[string]interface{}{
				"value": "${provider::k8sfn::parse_yaml(local.yaml)}",
			},
		},
	}

	configJson, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8sfn": providerserver.NewProtocol6WithError(fn.New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: string(configJson),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue(
						"yaml",
						knownvalue.ListExact(
							[]knownvalue.Check{
								knownvalue.StringExact("one"),
								knownvalue.StringExact("two"),
							},
						),
					),
				},
			},
		},
	})
}
