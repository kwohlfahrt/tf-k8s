//go:generate openapi example.com
package crd

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
	acmetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/dynamic"
)

type Clients struct {
	dynamic   *dynamic.DynamicClient
	extractor acmetav1.UnstructuredExtractor
}

type CrdProvider struct {
	version   string
	typeInfos []generic.TypeInfo
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

	dynamic, err := provider.MakeDynamicClient([]byte(data.Kubeconfig.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Unable to make kubernetes client", err.Error())
		return
	}

	discovery, err := provider.MakeDiscoveryClient([]byte(data.Kubeconfig.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Unable to make discovery client", err.Error())
		return
	}

	extractor, err := acmetav1.NewUnstructuredExtractor(discovery)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure extractor", err.Error())
		return
	}

	clients := Clients{dynamic: dynamic, extractor: extractor}
	resp.DataSourceData = clients
	resp.ResourceData = clients
}

func (p *CrdProvider) DataSources(context.Context) []func() datasource.DataSource {
	result := make([]func() datasource.DataSource, 0, len(p.typeInfos))
	for _, typeInfo := range p.typeInfos {
		result = append(result, func() datasource.DataSource { return NewDataSource(typeInfo) })
	}
	return result
}

func (p *CrdProvider) Resources(context.Context) []func() resource.Resource {
	result := make([]func() resource.Resource, 0, len(p.typeInfos))
	for _, typeInfo := range p.typeInfos {
		result = append(result, func() resource.Resource { return NewResource(typeInfo) })
	}
	return result
}

func (p *CrdProvider) Functions(context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) (func() tfprovider.Provider, error) {
	return func() tfprovider.Provider {
		return &CrdProvider{version: version, typeInfos: TypeInfos}
	}, nil
}
