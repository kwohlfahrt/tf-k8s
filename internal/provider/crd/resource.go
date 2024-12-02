package crd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type crdResource struct {
	typeInfo       generic.TypeInfo
	client         *dynamic.DynamicClient
	forceConflicts bool
}

func NewResource(typeInfo generic.TypeInfo) tfresource.Resource {
	return &crdResource{typeInfo: typeInfo}
}

func (c *crdResource) Metadata(ctx context.Context, req tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	groupComponents := []string{}
	if c.typeInfo.Group != "" {
		for _, component := range strings.Split(c.typeInfo.Group, ".") {
			groupComponents = append(groupComponents, strings.Replace(component, "-", "", -1))
		}
	}
	nameComponents := []string{req.ProviderTypeName, strings.ToLower(c.typeInfo.Kind)}
	nameComponents = append(nameComponents, groupComponents...)
	nameComponents = append(nameComponents, c.typeInfo.Version)
	resp.TypeName = strings.Join(nameComponents, "_")
}

func (c *crdResource) Schema(ctx context.Context, req tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	result, err := generic.OpenApiToTfSchema(ctx, c.typeInfo, false)
	if err != nil {
		resp.Diagnostics.AddError("Could not convert CRD to schema", err.Error())
		return
	}

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"manifest": result,
			"field_manager": schema.StringAttribute{
				Required: false,
				Computed: true,
				Default:  stringdefault.StaticString(fieldManager),
			},
		},
	}
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
	c.forceConflicts = clients.forceConflicts
}

const fieldManager string = "tofu-k8scrd"

func (c *crdResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var meta generic.ObjectMeta
	resp.Diagnostics.Append(generic.StateToObjectMeta(ctx, req.Plan, c.typeInfo, &meta)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state types.KubernetesObjectValue
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("manifest"), &state)...)
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
	obj, err := c.typeInfo.Interface(c.client, meta.Namespace).
		Apply(ctx, meta.Name, planObj, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
		return
	}

	fields, diags := generic.GetManagedFieldSet(obj, fieldManager)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(state.ManagedFields(ctx, path.Empty(), fields, nil)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(generic.UnstructuredToValue(ctx, c.typeInfo.Schema, *obj, fields.Leaves(), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("manifest"), state)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("field_manager"), fieldManager)...)
}

func (c *crdResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var meta generic.ObjectMeta
	resp.Diagnostics.Append(generic.StateToObjectMeta(ctx, req.State, c.typeInfo, &meta)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state types.KubernetesObjectValue
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("manifest"), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	importFieldManager, diags := generic.GetImportFieldManager(ctx, req.Private, "import-field-managers")
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	obj, err := c.typeInfo.Interface(c.client, meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
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
	resp.Diagnostics.Append(state.ManagedFields(ctx, path.Empty(), fields, nil)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(generic.UnstructuredToValue(ctx, c.typeInfo.Schema, *obj, fields.Leaves(), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("manifest"), state)...)
	if importFieldManager != nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("field_manager"), importFieldManager)...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("field_manager"), fieldManager)...)
	}
}

func (c *crdResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var meta generic.ObjectMeta
	resp.Diagnostics.Append(generic.StateToObjectMeta(ctx, req.State, c.typeInfo, &meta)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state types.KubernetesObjectValue
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("manifest"), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	importFieldManager, diags := generic.GetImportFieldManager(ctx, req.Private, "import-field-managers")
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	obj, diags := generic.ValueToUnstructured(ctx, state, c.typeInfo)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: Validate that the object already exists. This will silently create
	// the object if it does not already exist.
	obj, err := c.typeInfo.Interface(c.client, meta.Namespace).
		Apply(ctx, meta.Name, obj, metav1.ApplyOptions{FieldManager: fieldManager, Force: c.forceConflicts})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
		return
	}

	if importFieldManager != nil {
		empty := unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": c.typeInfo.GroupVersionResource().GroupVersion().String(),
			"kind":       c.typeInfo.Kind,
			"metadata":   map[string]interface{}{"name": meta.Name, "namespace": meta.Namespace},
		}}
		_, err := c.typeInfo.Interface(c.client, meta.Namespace).
			Apply(ctx, meta.Name, &empty, metav1.ApplyOptions{FieldManager: *importFieldManager})
		if err != nil {
			resp.Diagnostics.AddError("Unable to remove imported fieldManager", err.Error())
			return
		}
		resp.Private.SetKey(ctx, "import-field-managers", nil)
	}

	fields, diags := generic.GetManagedFieldSet(obj, fieldManager)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.Diagnostics.Append(state.ManagedFields(ctx, path.Empty(), fields, nil)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(generic.UnstructuredToValue(ctx, c.typeInfo.Schema, *obj, fields.Leaves(), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("manifest"), state)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("field_manager"), fieldManager)...)
}

func (c *crdResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var meta generic.ObjectMeta
	resp.Diagnostics.Append(generic.StateToObjectMeta(ctx, req.State, c.typeInfo, &meta)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fieldSelector := fmt.Sprintf("metadata.name=%s", meta.Name)
	w, err := c.client.Resource(c.typeInfo.GroupVersionResource()).
		Watch(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		resp.Diagnostics.AddError("Unable to watch resource for deletion", err.Error())
		return
	}
	defer w.Stop()

	err = c.typeInfo.Interface(c.client, meta.Namespace).Delete(ctx, meta.Name, metav1.DeleteOptions{})
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

	fieldManagerState, err := json.Marshal(fieldManagers)
	if err != nil {
		resp.Diagnostics.AddError("Unable to marshal fieldManagers", err.Error())
		return
	}

	metadata := make(map[string]interface{})
	if c.typeInfo.Namespaced {
		components = strings.SplitN(resource, "/", 2)
		if len(components) != 2 {
			resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected namespace/name resource, got %s", resource))
			return
		}
		metadata["namespace"] = components[0]
		metadata["name"] = components[1]
	} else {
		metadata["name"] = resource
	}

	obj := unstructured.Unstructured{Object: map[string]interface{}{"metadata": metadata}}
	var state types.KubernetesObjectValue
	resp.Diagnostics.Append(generic.UnstructuredToValue(ctx, c.typeInfo.Schema, obj, nil, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("manifest"), state)...)
	resp.Private.SetKey(ctx, "import-field-managers", fieldManagerState)
}

var (
	_ tfresource.Resource                = &crdResource{}
	_ tfresource.ResourceWithConfigure   = &crdResource{}
	_ tfresource.ResourceWithImportState = &crdResource{}
)
