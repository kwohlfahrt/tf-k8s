package crd_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/kwohlfahrt/tf-k8s/internal/provider"
	"github.com/kwohlfahrt/tf-k8s/internal/provider/crd"
)

func providerFactory(t *testing.T) map[string]func() (tfprotov6.ProviderServer, error) {
	providerFactory, err := crd.New("test")
	if err != nil {
		t.Fatal(err)
	}
	return map[string]func() (tfprotov6.ProviderServer, error){
		"k8s": providerserver.NewProtocol6WithError(providerFactory()),
	}
}

func TestAccResource(t *testing.T) {
	kubeconfig, err := os.ReadFile(os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatal(err)
	}

	k, err := provider.MakeDynamicClient(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}

	rawCheckSpec, err := os.ReadFile(fmt.Sprintf("fixtures/%s/test.json", os.Getenv("PROVIDER")))
	if err != nil {
		t.Fatal(err)
	}
	var checkSpeck checkSpec
	if err = json.Unmarshal(rawCheckSpec, &checkSpeck); err != nil {
		t.Fatal(err)
	}

	cfg := fmt.Sprintf("./fixtures/%s/test.tf", os.Getenv("PROVIDER"))
	protoV6ProviderFactories := providerFactory(t)
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			dataPath := fmt.Sprintf("./fixtures/%s/data.yaml", os.Getenv("PROVIDER"))
			cmd := exec.Command("kubectl", "apply", "--server-side", "-f", dataPath)
			if err := cmd.Run(); err != nil {
				t.Fatal(err)
			}
		},
		Steps: []resource.TestStep{
			{
				ProtoV6ProviderFactories: protoV6ProviderFactories,
				ConfigFile:               config.StaticFile(cfg),
				ConfigVariables:          config.Variables{"kubeconfig": config.StringVariable(string(kubeconfig))},
				Check:                    makeChecks(k, checkSpeck.Resources),
				ConfigPlanChecks:         makeConfigChecks(checkSpeck.Properties, checkSpeck.Outputs),
				ConfigStateChecks:        makeStateChecks(checkSpeck.State),
			},
			{
				ProtoV6ProviderFactories: protoV6ProviderFactories,
				ConfigFile:               config.StaticFile(cfg),
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

	failCfg := fmt.Sprintf("./fixtures/%s/fail.tf", os.Getenv("PROVIDER"))
	if _, err := os.Stat(failCfg); err != nil && os.IsNotExist(err) {
		t.Skip("No failure test configured")
	} else if err != nil {
		t.Fatal(err)
	}

	protoV6ProviderFactories := providerFactory(t)
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{{
			ProtoV6ProviderFactories: protoV6ProviderFactories,
			ConfigFile:               config.StaticFile(failCfg),
			ConfigVariables:          config.Variables{"kubeconfig": config.StringVariable(string(kubeconfig))},
			ExpectError:              regexp.MustCompile("already exists"),
		}},
	})
}

func patchState(wd string) error {
	matches, err := filepath.Glob(fmt.Sprintf("%s/*/terraform.tfstate", wd))
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("Expected exactly one state file, found %d", len(matches))
	}
	match := matches[0]

	rawState, err := os.ReadFile(match)
	if err != nil {
		return err
	}
	var state map[string]any
	if err = json.Unmarshal(rawState, &state); err != nil {
		return err
	}
	resources := state["resources"].([]any)
	for _, resource := range resources {
		resource := resource.(map[string]any)
		typ := resource["type"].(string)
		if typ, isK8s := strings.CutPrefix(typ, "k8s_"); isK8s {
			resource["type"] = "k8scrd_" + typ
		}
	}

	rawState, err = json.Marshal(state)
	if err != nil {
		return err
	}
	if err = os.WriteFile(match, rawState, 0); err != nil {
		return err
	}
	return nil
}

func TestMigration(t *testing.T) {
	kubeconfig, err := os.ReadFile(os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatal(err)
	}

	wd := t.TempDir()
	protoV6ProviderFactories := providerFactory(t)
	matches, err := filepath.Glob(fmt.Sprintf("./fixtures/%s/migrations/*.tf", os.Getenv("PROVIDER")))
	if len(matches) == 0 {
		t.Skip("No migration tests configured")
	}
	for _, cfg := range matches {
		if strings.HasSuffix(cfg, "-init.tf") {
			continue
		}

		initCfg := strings.TrimSuffix(cfg, ".tf") + "-init.tf"
		resource.Test(t, resource.TestCase{
			WorkingDir: wd,
			Steps: []resource.TestStep{
				{
					ProtoV6ProviderFactories: protoV6ProviderFactories,
					ConfigFile:               config.StaticFile(initCfg),
					ConfigVariables:          config.Variables{"kubeconfig": config.StringVariable(string(kubeconfig))},
				},
				{
					PreConfig:                func() { patchState(wd) },
					ProtoV6ProviderFactories: protoV6ProviderFactories,
					ConfigFile:               config.StaticFile(cfg),
					ConfigVariables: config.Variables{
						"kubeconfig": config.StringVariable(string(kubeconfig)),
						"update":     config.BoolVariable(true),
					},
				},
			},
		})
	}
}
