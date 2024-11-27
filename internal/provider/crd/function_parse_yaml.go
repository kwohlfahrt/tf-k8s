package crd

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
)

var _ function.Function = &ParseYAMLFunction{}

type ParseYAMLFunction struct {
	typeInfo generic.TypeInfo
}

func NewParseYAMLFunction(typeInfo generic.TypeInfo) function.Function {
	return &ParseYAMLFunction{typeInfo}
}

func (f *ParseYAMLFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	groupComponents := []string{}
	if f.typeInfo.Group != "" {
		groupComponents = strings.Split(f.typeInfo.Group, ".")
	}
	nameComponents := []string{"parse", strings.ToLower(f.typeInfo.Kind)}
	nameComponents = append(nameComponents, groupComponents...)
	nameComponents = append(nameComponents, f.typeInfo.Version)
	resp.Name = strings.Join(nameComponents, "_")
}

func (f *ParseYAMLFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Parse a multi-document YAML function into an array of objects",
		Description: "Given a multi-document YAML, parse it into an array of Kubernetes objects.",

		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:        "yaml",
				Description: "YAML document to parse",
				// TODO: Figure out why I can't use CustomType: types.KubernetesUnknownValue
			},
		},
		Return: function.ObjectReturn{
			AttributeTypes: f.typeInfo.Schema.AttrTypes,
			CustomType:     f.typeInfo.Schema,
		},
	}
}

func (f *ParseYAMLFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var yaml basetypes.DynamicValue

	resp.Error = req.Arguments.Get(ctx, &yaml)
	if resp.Error != nil {
		return
	}

	obj, diags := types.DynamicToUnstructured(yaml.UnderlyingValue(), path.Empty())
	if diags.HasError() {
		resp.Error = function.FuncErrorFromDiags(ctx, diags)
		return
	}

	value, valueDiags := f.typeInfo.Schema.ValueFromUnstructured(ctx, path.Empty(), nil, obj)
	diags.Append(valueDiags...)
	if diags.HasError() {
		resp.Error = function.FuncErrorFromDiags(ctx, diags)
		return
	}

	resp.Error = resp.Result.Set(ctx, value)
}
