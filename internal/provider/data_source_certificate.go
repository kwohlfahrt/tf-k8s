package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"

	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type certificateDataSource struct {
	client *dynamic.DynamicClient
}

func (c *certificateDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*dynamic.DynamicClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider data type",
			fmt.Sprintf("Expected *kubernetes.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	c.client = client
}

func NewCertificateDataSource() datasource.DataSource {
	return &certificateDataSource{}
}

func (c *certificateDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (c *certificateDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	crd, err := generic.LoadCrd(schemaBytes, "v1")
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse schema file", err.Error())
		return
	}
	if crd == nil {
		resp.Diagnostics.AddError("CRD version not found", "v1")
		return
	}

	result, err := generic.OpenApiToTfSchema(crd, true)
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

func (c *certificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var metadata objectMeta
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, err := c.client.Resource(certificateGvr).Namespace(metadata.Namespace).
		Get(ctx, metadata.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsGone(err) || errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch resource", err.Error())
		return
	}

	state, diags := generic.ObjectToState(ctx, resp.State, obj)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

var (
	_ datasource.DataSource              = &certificateDataSource{}
	_ datasource.DataSourceWithConfigure = &certificateDataSource{}
)
