package fn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	generictypes "github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

var _ function.Function = &ParseYAMLFunction{}

type ParseYAMLFunction struct{}

func NewParseYAMLFunction() function.Function {
	return &ParseYAMLFunction{}
}

func (f *ParseYAMLFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "parse_yaml"
}

func (f *ParseYAMLFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Parse a multi-document YAML function into an array of objects",
		Description: "Given a multi-document YAML, parse it into an array of Kubernetes objects.",

		Parameters: []function.Parameter{
			function.StringParameter{
				Name:        "yaml",
				Description: "YAML document to parse",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (f *ParseYAMLFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var yaml string

	resp.Error = req.Arguments.Get(ctx, &yaml)
	if resp.Error != nil {
		return
	}
	yamlReader := strings.NewReader(yaml)
	decoder := utilyaml.NewYAMLOrJSONDecoder(yamlReader, 4096)

	var diags diag.Diagnostics
	data := []attr.Value{}
	types := []attr.Type{}

	t := generictypes.KubernetesUnknownType{}

document:
	for i := 0; ; i += 1 {
		itemPath := path.Empty().AtListIndex(i)
		item := map[string]interface{}{}
		err := decoder.Decode(&item)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			resp.Error = function.NewArgumentFuncError(0, fmt.Sprintf("Unable to decode input YAML: %s", err.Error()))
			return
		}
		for _, k := range []string{"apiVersion", "kind", "metadata"} {
			if _, found := item[k]; !found {
				diags.Append(diag.NewAttributeErrorDiagnostic(
					itemPath, "Missing required field",
					fmt.Sprintf("Item does not define the property %s", k),
				))
				continue document
			}
		}

		obj, itemDiags := t.ValueFromUnstructured(ctx, itemPath, nil, item)
		diags.Append(itemDiags...)
		if itemDiags.HasError() {
			continue
		}

		data = append(data, obj)
		types = append(types, obj.Type(ctx))
	}

	tfData, tfDiags := basetypes.NewTupleValue(types, data)
	diags.Append(tfDiags...)

	resp.Error = function.FuncErrorFromDiags(ctx, diags)
	if resp.Error != nil {
		return
	}

	resp.Error = resp.Result.Set(ctx, basetypes.NewDynamicValue(tfData))
}
