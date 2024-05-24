package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

type certificateResource struct {
	client *dynamic.DynamicClient
}

func NewCertificateResource() resource.Resource {
	return &certificateResource{}
}

func (c *certificateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (c *certificateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				Required: true,
				Attributes: map[string]schema.Attribute{
					"dns_names": schema.ListAttribute{
						ElementType: types.StringType,
						Required:    true,
					},
					"issuer_ref": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"group": schema.StringAttribute{Required: true},
							"kind":  schema.StringAttribute{Required: true},
							"name":  schema.StringAttribute{Required: true},
						},
						Required: true,
					},
					"secret_name": schema.StringAttribute{Required: true},
				},
			},
		},
	}
}

func (c *certificateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

const fieldManager string = "tofu-k8scrd"

func (c *certificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data certificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certificate := dumpCertificate(&data)

	// TODO: Validate that we haven't previously created the object. It will
	// conflict fail if it was created by a different tool, but if we created it
	// and forgot, this will silently adopt the object.  We could generate a
	// unique `FieldManager` ID per resource, and persist it in the TF state.
	obj, err := c.client.Resource(certificateGvr).
		Namespace(data.Metadata.Namespace).
		Apply(ctx, data.Metadata.Name, certificate, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
	}

	state, err := loadCertificate(obj)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse resource", err.Error())
		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (c *certificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data certificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, err := c.client.Resource(certificateGvr).
		Namespace(data.Metadata.Namespace).
		Get(ctx, data.Metadata.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsGone(err) || errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch resource", err.Error())
		return
	}

	state, err := loadCertificate(obj)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse resource", err.Error())
		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (c *certificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data certificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certificate := dumpCertificate(&data)
	obj, err := c.client.Resource(certificateGvr).
		Namespace(data.Metadata.Namespace).
		Apply(ctx, data.Metadata.Name, certificate, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to patch resource", err.Error())
		return
	}

	state, err := loadCertificate(obj)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse resource", err.Error())
		return
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (c *certificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data certificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := c.client.Resource(certificateGvr).
		Namespace(data.Metadata.Namespace).
		Delete(ctx, data.Metadata.Name, metav1.DeleteOptions{})
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete resource", err.Error())
		return
	}
}

var (
	_ resource.Resource              = &certificateResource{}
	_ resource.ResourceWithConfigure = &certificateResource{}
)
