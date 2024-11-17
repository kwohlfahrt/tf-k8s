package crd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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
	groupComponents := []string{}
	if c.typeInfo.Group != "" {
		groupComponents = strings.Split(c.typeInfo.Group, ".")
	}
	nameComponents := []string{req.ProviderTypeName, strings.ToLower(c.typeInfo.Kind)}
	nameComponents = append(nameComponents, groupComponents...)
	nameComponents = append(nameComponents, c.typeInfo.Version)
	resp.TypeName = strings.Join(nameComponents, "_")
}

func (c *crdResource) Schema(ctx context.Context, req tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	result, err := generic.OpenApiToTfSchema(ctx, c.typeInfo.Schema)
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

	clients, ok := req.ProviderData.(Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider data type",
			fmt.Sprintf("Expected *kubernetes.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	c.client = clients.dynamic
}

const fieldManager string = "tofu-k8scrd"

func (c *crdResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var name, namespace string
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata").AtName("name"), &name)...)
	if c.typeInfo.Namespaced {
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata").AtName("namespace"), &namespace)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	state, diags := generic.StateToValue(ctx, req.Plan, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	planObj, diags := generic.ValueToUnstructured(ctx, state, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: Validate that we haven't previously created the object. It will
	// conflict fail if it was created by a different tool, but if we created it
	// and forgot, this will silently adopt the object. We could generate a
	// unique `FieldManager` ID per resource, and persist it in the TF state.
	obj, err := c.typeInfo.Interface(c.client, namespace).Apply(ctx, name, planObj, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
		return
	}

	fields, diags := generic.GetManagedFieldSet(obj, fieldManager)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	diags = state.ManagedFields(ctx, path.Empty(), fields, nil)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	state, diags = generic.UnstructuredToValue(ctx, c.typeInfo.Schema, *obj, fields.Leaves())
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (c *crdResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var name, namespace string
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata").AtName("name"), &name)...)
	if c.typeInfo.Namespaced {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata").AtName("namespace"), &namespace)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	state, diags := generic.StateToValue(ctx, req.State, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	importFieldManager, diags := generic.GetImportFieldManager(ctx, req.Private, "import-field-managers")
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	obj, err := c.typeInfo.Interface(c.client, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsGone(err) || errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to fetch resource", err.Error())
		return
	}

	readFieldManager := fieldManager
	if importFieldManager != nil {
		readFieldManager = *importFieldManager
	}

	fields, diags := generic.GetManagedFieldSet(obj, readFieldManager)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	diags = state.ManagedFields(ctx, path.Empty(), fields, nil)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	state, diags = generic.UnstructuredToValue(ctx, c.typeInfo.Schema, *obj, fields.Leaves())
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (c *crdResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var name, namespace string
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata").AtName("name"), &name)...)
	if c.typeInfo.Namespaced {
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("metadata").AtName("namespace"), &namespace)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	state, diags := generic.StateToValue(ctx, req.Plan, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	obj, diags := generic.ValueToUnstructured(ctx, state, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: Validate that the object already exists. This will silently create
	// the object if it does not already exist.
	obj, err := c.typeInfo.Interface(c.client, namespace).Apply(ctx, name, obj, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
		return
	}

	// TODO: Should we unset the existing field-manager? Should we
	// unconditionally do that on import (currently, Update only runs if there
	// are changes)?
	resp.Private.SetKey(ctx, "import-field-managers", nil)

	fields, diags := generic.GetManagedFieldSet(obj, fieldManager)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	diags = state.ManagedFields(ctx, path.Empty(), fields, nil)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	state, diags = generic.UnstructuredToValue(ctx, c.typeInfo.Schema, *obj, fields.Leaves())
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (c *crdResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var name, namespace string
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata").AtName("name"), &name)...)
	if c.typeInfo.Namespaced {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata").AtName("namespace"), &namespace)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	fieldSelector := fmt.Sprintf("metadata.name=%s", name)
	w, err := c.client.Resource(c.typeInfo.GroupVersionResource()).Watch(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		resp.Diagnostics.AddError("Unable to watch resource for deletion", err.Error())
		return
	}
	defer w.Stop()

	err = c.typeInfo.Interface(c.client, namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete resource", err.Error())
		return
	}

	for event := range w.ResultChan() {
		if event.Type == watch.Deleted {
			break
		}
	}
}

func (c *crdResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	components := strings.SplitN(req.ID, ":", 2)
	if len(components) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected fieldManagers:resource import, got %s", req.ID))
		return
	}

	fieldManagers := strings.Split(components[0], ",")
	resource := components[1]

	state, err := json.Marshal(fieldManagers)
	if err != nil {
		resp.Diagnostics.AddError("Unable to marshal fieldManagers", err.Error())
		return
	}

	if c.typeInfo.Namespaced {
		components = strings.SplitN(resource, "/", 2)
		if len(components) != 2 {
			resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected namespace/name resource, got %s", resource))
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("metadata").AtName("namespace"), components[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("metadata").AtName("name"), components[1])...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("metadata").AtName("name"), resource)...)
	}

	resp.Private.SetKey(ctx, "import-field-managers", state)
}

var (
	_ tfresource.Resource                = &crdResource{}
	_ tfresource.ResourceWithConfigure   = &crdResource{}
	_ tfresource.ResourceWithImportState = &crdResource{}
)
