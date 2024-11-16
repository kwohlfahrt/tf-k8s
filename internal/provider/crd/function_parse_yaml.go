package crd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
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

	obj, diags := toUnstructured(yaml.UnderlyingValue(), path.Empty())
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

func toUnstructured(obj attr.Value, path path.Path) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	switch v := obj.(type) {
	case basetypes.ObjectValue:
		attrs := v.Attributes()
		unstructured := make(map[string]interface{}, len(attrs))
		for k, v := range attrs {
			attrPath := path.AtName(k)
			attr, attrDiags := toUnstructured(v, attrPath)
			diags.Append(attrDiags...)
			if !attrDiags.HasError() {
				unstructured[k] = attr
			}
		}
		return unstructured, diags
	case basetypes.TupleValue:
		elems := v.Elements()
		unstructured := make([]interface{}, 0, len(elems))
		for i, v := range elems {
			elemPath := path.AtTupleIndex(i)
			elem, elemDiags := toUnstructured(v, elemPath)
			diags.Append(elemDiags...)
			if !elemDiags.HasError() {
				unstructured = append(unstructured, elem)
			}
		}
		return unstructured, diags
	case basetypes.ListValue:
		elems := v.Elements()
		unstructured := make([]interface{}, 0, len(elems))
		for i, v := range elems {
			elemPath := path.AtListIndex(i)
			elem, elemDiags := toUnstructured(v, elemPath)
			diags.Append(elemDiags...)
			if !elemDiags.HasError() {
				unstructured = append(unstructured, elem)
			}
		}
		return unstructured, diags
	case basetypes.MapValue:
		elems := v.Elements()
		unstructured := make(map[string]interface{}, len(elems))
		for k, v := range elems {
			attrPath := path.AtMapKey(k)
			attr, attrDiags := toUnstructured(v, attrPath)
			diags.Append(attrDiags...)
			if !attrDiags.HasError() {
				unstructured[k] = attr
			}
		}
		return unstructured, diags
	case basetypes.SetValue:
		elems := v.Elements()
		unstructured := make([]interface{}, 0, len(elems))
		for _, v := range elems {
			elemPath := path.AtSetValue(v)
			elem, elemDiags := toUnstructured(v, elemPath)
			diags.Append(elemDiags...)
			if !elemDiags.HasError() {
				unstructured = append(unstructured, elem)
			}
		}
		return unstructured, diags
	case basetypes.StringValue:
		return v.ValueString(), diags
	case basetypes.NumberValue:
		f := v.ValueBigFloat()
		if f.IsInt() {
			i, _ := f.Int64()
			return i, diags
		} else {
			f, _ := f.Float64()
			return f, diags
		}
	case basetypes.BoolValue:
		return v.ValueBool(), diags
	default:
		diags.Append(diag.NewAttributeErrorDiagnostic(path, "Unsupported dynamic value type", fmt.Sprintf("got %T", v)))
		return nil, diags
	}
}
