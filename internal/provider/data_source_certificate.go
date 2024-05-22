package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"k8s.io/client-go/kubernetes"
)

type certificateDataSourceModel struct {
	Spec certificateSpec `tfsdk:"spec"`
}

type certificateSpec struct {
	DnsNames []types.String `tfsdk:"dns_names"`
}

type certificateDataSource struct {
	client *kubernetes.Clientset
}

func (c *certificateDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*kubernetes.Clientset)
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
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"spec": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"dns_names": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
					},
				},
			},
		},
	}
}

func (c *certificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	state := certificateDataSourceModel{
		Spec: certificateSpec{
			DnsNames: []types.String{types.StringValue("example.com")},
		},
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

var (
	_ datasource.DataSource              = &certificateDataSource{}
	_ datasource.DataSourceWithConfigure = &certificateDataSource{}
)
