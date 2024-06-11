package generic

import (
	"context"
	_ "embed"

	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

//go:embed test.crds.yaml
var schemaBytes []byte

func TestRequiredFields(t *testing.T) {
	crd, err := LoadCrd(schemaBytes, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if crd == nil {
		t.Fatal("CRD version not found: v1")
	}

	result, err := OpenApiToTfSchema(context.Background(), crd, false)
	if err != nil {
		t.Fatal(err)
	}
	spec := result.Attributes["spec"].(schema.SingleNestedAttribute)
	if !spec.Attributes["x"].IsRequired() {
		t.Error("Expected attribute spec.x to be required")
	}
	if spec.Attributes["y"].IsRequired() {
		t.Error("Expected attribute spec.x to not be required")
	}
}
