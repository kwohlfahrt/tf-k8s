package fn

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		Return: function.ListReturn{ElementType: types.StringType},
	}
}

func (f *ParseYAMLFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var diags diag.Diagnostics
	var yaml string

	resp.Error = req.Arguments.Get(ctx, &yaml)
	if resp.Error != nil {
		return
	}

	result := []string{"one", "two"}

	resp.Error = function.FuncErrorFromDiags(ctx, diags)
	if resp.Error != nil {
		return
	}

	resp.Error = resp.Result.Set(ctx, result)
}
