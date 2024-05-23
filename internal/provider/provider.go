package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type CrdProvider struct {
	version string
}

type CrdProviderModel struct {
	Kubeconfig types.String `tfsdk:"kubeconfig"`
}

func (p *CrdProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tfcrd"
	resp.Version = p.version
}

func (p *CrdProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"kubeconfig": schema.StringAttribute{
				MarkdownDescription: "Kubernetes Configuration",
				Required:            true,
			},
		},
	}
}

func (p *CrdProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data CrdProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc, err := clientcmd.NewClientConfigFromBytes([]byte(data.Kubeconfig.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Unable to load kubeconfig", err.Error())
		return
	}
	cfg, err := cc.ClientConfig()
	if err != nil {
		resp.Diagnostics.AddError("Unable to get client configuration", err.Error())
		return
	}
	k, err := dynamic.NewForConfig(cfg)
	if err != nil {
		resp.Diagnostics.AddError("Unable to construct kubernetes client", err.Error())
		return
	}

	resp.DataSourceData = k
	resp.ResourceData = k
}

func (p *CrdProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewCertificateDataSource,
	}
}

func (p *CrdProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *CrdProvider) Functions(context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CrdProvider{
			version: version,
		}
	}
}
