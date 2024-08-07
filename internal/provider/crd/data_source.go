package crd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"

	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type crdDataSource struct {
	typeInfo generic.TypeInfo
	client   *dynamic.DynamicClient
}

func (c *crdDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider data type",
			fmt.Sprintf("Expected *kubernetes.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	c.client = clients.dynamic
}

func NewDataSource(typeInfo generic.TypeInfo) datasource.DataSource {
	return &crdDataSource{typeInfo: typeInfo}
}

func (c *crdDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	groupComponents := []string{}
	if c.typeInfo.Group != "" {
		groupComponents = strings.Split(c.typeInfo.Group, ".")
	}
	nameComponents := []string{req.ProviderTypeName, strings.ToLower(c.typeInfo.Kind)}
	nameComponents = append(nameComponents, groupComponents...)
	nameComponents = append(nameComponents, c.typeInfo.Version)
	resp.TypeName = strings.Join(nameComponents, "_")
}

func (c *crdDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	result, err := generic.OpenApiToTfSchema(ctx, c.typeInfo.Schema)
	if err != nil {
		resp.Diagnostics.AddError("Could not convert CRD to schema", err.Error())
		return
	}

	attributes := make(map[string]schema.Attribute, len(result.Attributes))
	for name, attr := range result.Attributes {
		// resource attributes and datasource attributes are the same interface
		// (fwschema.Attribute), so just cast it. Not sure if there's a cleaner
		// way to implement this.
		attributes[name] = attr.(schema.Attribute)
	}

	resp.Schema = schema.Schema{
		Attributes:          attributes,
		Description:         result.Description,
		MarkdownDescription: result.MarkdownDescription,
	}
}

func (c *crdDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var name, namespace string
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("metadata").AtName("name"), &name)...)
	if c.typeInfo.Namespaced {
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("metadata").AtName("namespace"), &namespace)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	obj, err := c.typeInfo.Interface(c.client, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsGone(err) || errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch resource", err.Error())
		return
	}

	state, diags := c.typeInfo.Schema.ValueFromUnstructured(ctx, path.Empty(), obj.UnstructuredContent())
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

var (
	_ datasource.DataSource              = &crdDataSource{}
	_ datasource.DataSourceWithConfigure = &crdDataSource{}
)
