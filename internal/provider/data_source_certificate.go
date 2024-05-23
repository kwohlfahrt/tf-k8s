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
					"issuer_ref": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"group": schema.StringAttribute{Computed: true},
							"kind":  schema.StringAttribute{Computed: true},
							"name":  schema.StringAttribute{Computed: true},
						},
						Computed: true,
					},
					"secret_name": schema.StringAttribute{Computed: true},
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
	if err != nil || !found {
		if err == nil {
			err = fmt.Errorf("field spec.dnsNames not found")
		}
		resp.Diagnostics.AddError("Error extracting field", err.Error())
		return
	}

	secretName, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "secretName")
	if err != nil || !found {
		if err == nil {
			err = fmt.Errorf("field spec.secretName not found")
		}
		resp.Diagnostics.AddError("Error extracting field", err.Error())
		return
	}

	issuerGroup, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "issuerRef", "group")
	if err != nil || !found {
		if err == nil {
			err = fmt.Errorf("field spec.issuerRef.group not found")
		}
		resp.Diagnostics.AddError("Error extracting field", err.Error())
		return
	}

	issuerKind, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "issuerRef", "kind")
	if err != nil || !found {
		if err == nil {
			err = fmt.Errorf("field spec.issuerRef.kind not found")
		}
		resp.Diagnostics.AddError("Error extracting field", err.Error())
		return
	}

	issuerName, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "issuerRef", "name")
	if err != nil || !found {
		if err == nil {
			err = fmt.Errorf("field spec.issuerRef.name not found")
		}
		resp.Diagnostics.AddError("Error extracting field", err.Error())
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
		Spec: &certificateSpec{
			DnsNames:   dnsNames,
			IssuerRef:  issuerRef{Group: issuerGroup, Kind: issuerKind, Name: issuerName},
			SecretName: secretName,
		},
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

var (
	_ datasource.DataSource              = &certificateDataSource{}
	_ datasource.DataSourceWithConfigure = &certificateDataSource{}
)
