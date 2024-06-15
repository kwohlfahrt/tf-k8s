package fn

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type FnProvider struct {
	version string
}

func (p *FnProvider) Metadata(ctx context.Context, req tfprovider.MetadataRequest, resp *tfprovider.MetadataResponse) {
	resp.TypeName = "k8sfn"
	resp.Version = p.version
}

func (p *FnProvider) Schema(ctx context.Context, req tfprovider.SchemaRequest, resp *tfprovider.SchemaResponse) {
}

func (p *FnProvider) Configure(ctx context.Context, req tfprovider.ConfigureRequest, resp *tfprovider.ConfigureResponse) {
}

func (p *FnProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *FnProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *FnProvider) Functions(context.Context) []func() function.Function {
	return []func() function.Function{
		NewParseYAMLFunction,
	}
}

func New(version string) func() tfprovider.Provider {
	return func() tfprovider.Provider {
		provider := FnProvider{version: version}
		return &provider
	}
}
