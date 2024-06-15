package crd

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
)

type CrdProvider struct {
	version  string
	typeInfo generic.TypeInfo
}

type CrdProviderModel struct {
	Kubeconfig types.String `tfsdk:"kubeconfig"`
}

func (p *CrdProvider) Metadata(ctx context.Context, req tfprovider.MetadataRequest, resp *tfprovider.MetadataResponse) {
	resp.TypeName = "k8scrd"
	resp.Version = p.version
}

func (p *CrdProvider) Schema(ctx context.Context, req tfprovider.SchemaRequest, resp *tfprovider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"kubeconfig": schema.StringAttribute{
				MarkdownDescription: "Kubernetes Configuration",
				Required:            true,
			},
		},
	}
}

func (p *CrdProvider) Configure(ctx context.Context, req tfprovider.ConfigureRequest, resp *tfprovider.ConfigureResponse) {
	var data CrdProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	k, err := provider.MakeDynamicClient([]byte(data.Kubeconfig.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Unable to make kubernetes client", err.Error())
		return
	}

	resp.DataSourceData = k
	resp.ResourceData = k
}

func (p *CrdProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource { return NewDataSource(p.typeInfo) },
	}
}

func (p *CrdProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return NewResource(p.typeInfo) },
	}
}

func (p *CrdProvider) Functions(context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) (func() tfprovider.Provider, error) {
	typeInfo, err := generic.LoadCrd(internal.SchemaBytes, "v1")
	if err != nil {
		return nil, err
	}
	if typeInfo == nil {
		return nil, fmt.Errorf("CRD version v1 not found")
	}

	return func() tfprovider.Provider {
		return &CrdProvider{version: version, typeInfo: *typeInfo}
	}, nil
}
