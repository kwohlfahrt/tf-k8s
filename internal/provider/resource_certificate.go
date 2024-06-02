package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
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
	crd, err := generic.LoadCrd(schemaBytes, "v1")
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse schema file", err.Error())
		return
	}
	if crd == nil {
		resp.Diagnostics.AddError("CRD version not found", "v1")
		return
	}

	result, err := generic.OpenApiToTfSchema(crd, false)
	if err != nil {
		resp.Diagnostics.AddError("Could not convert CRD to schema", err.Error())
		return
	}

	resp.Schema = *result
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
	var metadata objectMeta
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, diags := generic.StateToObject(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: Validate that we haven't previously created the object. It will
	// conflict fail if it was created by a different tool, but if we created it
	// and forgot, this will silently adopt the object. We could generate a
	// unique `FieldManager` ID per resource, and persist it in the TF state.
	obj, err := c.client.Resource(certificateGvr).Namespace(metadata.Namespace).
		Apply(ctx, metadata.Name, obj, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
		return
	}

	state, diags := generic.ObjectToState(ctx, resp.State, obj)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (c *certificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var metadata objectMeta
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
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

func (c *certificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var metadata objectMeta
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, diags := generic.StateToObject(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: Validate that the object already exists. This will silently create
	// the object if it does not already exist.
	obj, err := c.client.Resource(certificateGvr).Namespace(metadata.Namespace).
		Apply(ctx, metadata.Name, obj, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
		return
	}

	state, diags := generic.ObjectToState(ctx, resp.State, obj)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (c *certificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var metadata objectMeta
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := c.client.Resource(certificateGvr).Namespace(metadata.Namespace).
		Delete(ctx, metadata.Name, metav1.DeleteOptions{})
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete resource", err.Error())
		return
	}
}

var (
	_ resource.Resource              = &certificateResource{}
	_ resource.ResourceWithConfigure = &certificateResource{}
)
