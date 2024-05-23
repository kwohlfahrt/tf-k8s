package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"k8s.io/client-go/dynamic"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
)

type certificateDataSourceModel struct {
	Metadata *certificateMetadata `tfsdk:"metadata"`
	Spec     *certificateSpec     `tfsdk:"spec"`
}

type certificateMetadata struct {
	Name      string `tfsdk:"name"`
	Namespace string `tfsdk:"namespace"`
}

type certificateSpec struct {
	DnsNames []string `tfsdk:"dns_names"`
}

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
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"metadata": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"name":      schema.StringAttribute{Required: true},
					"namespace": schema.StringAttribute{Required: true},
				},
			},
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

var certificateResource = runtimeschema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}

func (c *certificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data certificateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, err := c.client.Resource(certificateResource).
		Namespace(data.Metadata.Namespace).
		Get(ctx, data.Metadata.Name, metav1.GetOptions{})

	if err != nil {
		resp.Diagnostics.AddError("Unable to fetch resource", err.Error())
		return
	}

	dnsNames, found, err := unstructured.NestedStringSlice(obj.UnstructuredContent(), "spec", "dnsNames")
	if err != nil {
		resp.Diagnostics.AddError("Error extracting spec.dnsNames", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("No field found for spec.dnsNames", "")
		return
	}

	name, found, err := unstructured.NestedString(obj.UnstructuredContent(), "metadata", "name")
	if err != nil {
		resp.Diagnostics.AddError("Error extracting metadata.name", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("No field found for metadata.name", "")
		return
	}

	namespace, found, err := unstructured.NestedString(obj.UnstructuredContent(), "metadata", "namespace")
	if err != nil {
		resp.Diagnostics.AddError("Error extracting metadata.namespace", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("No field found for metadata.namespace", "")
		return
	}

	state := certificateDataSourceModel{
		Metadata: &certificateMetadata{Name: name, Namespace: namespace},
		Spec:     &certificateSpec{DnsNames: dnsNames},
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

var (
	_ datasource.DataSource              = &certificateDataSource{}
	_ datasource.DataSourceWithConfigure = &certificateDataSource{}
)
