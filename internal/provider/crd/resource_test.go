package crd_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/kwohlfahrt/tf-k8s/internal/provider"
	"github.com/kwohlfahrt/tf-k8s/internal/provider/crd"
)

func TestAccResource(t *testing.T) {
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

	rawCheckSpec, err := os.ReadFile(fmt.Sprintf("fixtures/%s/test.json", os.Getenv("PROVIDER")))
	if err != nil {
		t.Fatal(err)
	}
	var checkSpeck checkSpec
	err = json.Unmarshal(rawCheckSpec, &checkSpeck)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := os.ReadFile(fmt.Sprintf("./fixtures/%s/test.tf", os.Getenv("PROVIDER")))
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8s": providerserver.NewProtocol6WithError(providerFactory()),
		},
		PreCheck: func() {
			dataPath := fmt.Sprintf("./fixtures/%s/data.yaml", os.Getenv("PROVIDER"))
			cmd := exec.Command("kubectl", "apply", "--server-side", "-f", dataPath)
			if err := cmd.Run(); err != nil {
				t.Fatal(err)
			}
		},
		Steps: []resource.TestStep{
			{
				Config:            string(cfg),
				ConfigVariables:   config.Variables{"kubeconfig": config.StringVariable(string(kubeconfig))},
				Check:             makeChecks(k, checkSpeck.Resources),
				ConfigPlanChecks:  makeConfigChecks(checkSpeck.Properties, checkSpeck.Outputs),
				ConfigStateChecks: makeStateChecks(checkSpeck.State),
			},
			{
				Config: string(cfg),
				ConfigVariables: config.Variables{
					"kubeconfig": config.StringVariable(string(kubeconfig)),
					"update":     config.BoolVariable(true),
				},
			},
		},
		CheckDestroy: makeDestroyChecks(k, checkSpeck.Resources),
	})
}

func TestFail(t *testing.T) {
	kubeconfig, err := os.ReadFile(os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatal(err)
	}

	providerFactory, err := crd.New("test")
	if err != nil {
		t.Fatal(err)
	}

	failCfg, err := os.ReadFile(fmt.Sprintf("./fixtures/%s/fail.tf", os.Getenv("PROVIDER")))
	if err != nil && os.IsNotExist(err) {
		t.Skip("No failure test configured")
	} else if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"k8s": providerserver.NewProtocol6WithError(providerFactory()),
		},
		Steps: []resource.TestStep{{
			Config:          string(failCfg),
			ConfigVariables: config.Variables{"kubeconfig": config.StringVariable(string(kubeconfig))},
			ExpectError:     regexp.MustCompile("already exists"),
		}},
	})
}
