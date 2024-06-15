package crd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"github.com/stoewer/go-strcase"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

type crdResource struct {
	typeInfo generic.TypeInfo
	client   *dynamic.DynamicClient
}

func NewResource(typeInfo generic.TypeInfo) tfresource.Resource {
	return &crdResource{typeInfo: typeInfo}
}

func (c *crdResource) Metadata(ctx context.Context, req tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	groupComponents := strings.Split(c.typeInfo.Group, ".")
	nameComponents := []string{req.ProviderTypeName, strcase.SnakeCase(c.typeInfo.Kind)}

	nameComponents = append(nameComponents, groupComponents...)
	nameComponents = append(nameComponents, c.typeInfo.Version)
	resp.TypeName = strings.Join(nameComponents, "_")
}

func (c *crdResource) Schema(ctx context.Context, req tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	result, err := generic.OpenApiToTfSchema(ctx, c.typeInfo.Schema, false)
	if err != nil {
		resp.Diagnostics.AddError("Could not convert CRD to schema", err.Error())
		return
	}

	resp.Schema = *result
}

func (c *crdResource) Configure(ctx context.Context, req tfresource.ConfigureRequest, resp *tfresource.ConfigureResponse) {
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

func (c *crdResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var metadata types.ObjectMeta
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, diags := generic.StateToObject(ctx, req.Plan, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: Validate that we haven't previously created the object. It will
	// conflict fail if it was created by a different tool, but if we created it
	// and forgot, this will silently adopt the object. We could generate a
	// unique `FieldManager` ID per resource, and persist it in the TF state.
	obj, err := c.client.Resource(c.typeInfo.GroupVersionResource()).Namespace(metadata.Namespace).
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

func (c *crdResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var metadata types.ObjectMeta
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, err := c.client.Resource(c.typeInfo.GroupVersionResource()).Namespace(metadata.Namespace).
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

func (c *crdResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var metadata types.ObjectMeta
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, diags := generic.StateToObject(ctx, req.Plan, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: Validate that the object already exists. This will silently create
	// the object if it does not already exist.
	obj, err := c.client.Resource(c.typeInfo.GroupVersionResource()).Namespace(metadata.Namespace).
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

func (c *crdResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var metadata types.ObjectMeta
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata"), &metadata)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := c.client.Resource(c.typeInfo.GroupVersionResource()).Namespace(metadata.Namespace).
		Delete(ctx, metadata.Name, metav1.DeleteOptions{})
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete resource", err.Error())
		return
	}
}

var (
	_ tfresource.Resource              = &crdResource{}
	_ tfresource.ResourceWithConfigure = &crdResource{}
)
