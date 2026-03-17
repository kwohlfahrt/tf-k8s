package crd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/kwohlfahrt/tf-k8s/internal/generic"
	"github.com/kwohlfahrt/tf-k8s/internal/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/csaupgrade"
	"k8s.io/client-go/util/retry"
)

type crdResource struct {
	typeInfo generic.TypeInfo
	client   *dynamic.DynamicClient
}

func NewResource(typeInfo generic.TypeInfo) tfresource.Resource {
	return &crdResource{typeInfo: typeInfo}
}

func typeName(providerTypeName string, typeInfo generic.TypeInfo) string {
	groupComponents := []string{}
	if typeInfo.Group != "" {
		for _, component := range strings.Split(typeInfo.Group, ".") {
			groupComponents = append(groupComponents, strings.Replace(component, "-", "", -1))
		}
	}

	nameComponents := []string{providerTypeName, strings.ToLower(typeInfo.Kind)}
	nameComponents = append(nameComponents, groupComponents...)
	nameComponents = append(nameComponents, typeInfo.Version)
	return strings.Join(nameComponents, "_")
}

func (c *crdResource) Metadata(ctx context.Context, req tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = typeName(req.ProviderTypeName, c.typeInfo)
}

const defaultFieldManager string = "tofu-k8s"

func (c *crdResource) Schema(ctx context.Context, req tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"manifest": generic.OpenApiToTfSchema(ctx, c.typeInfo, false),
			"field_manager": schema.StringAttribute{
				Required: false,
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(defaultFieldManager),
			},
			"force_conflicts": schema.BoolAttribute{
				Required: false,
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
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
}

func (c *crdResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var meta generic.ObjectMeta
	resp.Diagnostics.Append(generic.StateToObjectMeta(ctx, req.Plan, c.typeInfo, &meta)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var fieldManager string
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("field_manager"), &fieldManager)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var forceConflicts *bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("force_conflicts"), &forceConflicts)...)
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

	iface := c.typeInfo.Interface(c.client, meta.Namespace)
	// Use a `Create` operation, to ensure we are creating the resource.  This
	// ensures two terraform operations can't both create the same object. See
	// also github.com/kubernetes/kubernetes#116156
	obj, err := iface.
		Create(ctx, planObj, metav1.CreateOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create resource", err.Error())
		return
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// We have created the object, so all fields are owned by `fieldManager`,
		// but with `operation: Update`. This produces a conflict when we try to
		// server-side apply changes in `Update()`. Use `csaupgrade` to migrate the
		// field manager to `operation: Apply`. This might be better in `Update()`.
		patchData, err := csaupgrade.UpgradeManagedFieldsPatch(obj, sets.New(fieldManager), fieldManager)
		if err != nil {
			return err
		}

		obj, err = iface.Patch(ctx, meta.Name, k8stypes.JSONPatchType, patchData, metav1.PatchOptions{})
		if err != nil && errors.IsConflict(err) {
			newObj, getErr := iface.Get(ctx, meta.Name, metav1.GetOptions{})
			if getErr != nil {
				return getErr
			}
			obj = newObj
		}
		return err
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to patch field-manager", err.Error())
		return
	}

	// Apply to update the list of tracked fields to match `planObj`, not the entire object creation.
	obj, err = iface.Apply(ctx, meta.Name, planObj, metav1.ApplyOptions{FieldManager: fieldManager})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update resource", err.Error())
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force_conflicts"), forceConflicts)...)
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
	var fieldManager string
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("field_manager"), &fieldManager)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var forceConflicts *bool
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("force_conflicts"), &forceConflicts)...)
	if resp.Diagnostics.HasError() {
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force_conflicts"), forceConflicts)...)
}

func (c *crdResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var meta generic.ObjectMeta
	resp.Diagnostics.Append(generic.StateToObjectMeta(ctx, req.State, c.typeInfo, &meta)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var fieldManager string
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("field_manager"), &fieldManager)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var planFieldManager string
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("field_manager"), &planFieldManager)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var forceConflicts *bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("force_conflicts"), &forceConflicts)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if fieldManager != planFieldManager {
		var oldState types.KubernetesObjectValue
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("manifest"), &oldState)...)
		if resp.Diagnostics.HasError() {
			return
		}
		obj, diags := generic.ValueToUnstructured(ctx, oldState, c.typeInfo)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		_, err := c.typeInfo.Interface(c.client, meta.Namespace).
			Apply(ctx, meta.Name, obj, metav1.ApplyOptions{FieldManager: planFieldManager, Force: *forceConflicts})
		if err != nil {
			resp.Diagnostics.AddError("Unable to apply new fieldManager", err.Error())
			return
		}

		empty := unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": c.typeInfo.GroupVersionResource().GroupVersion().String(),
			"kind":       c.typeInfo.Kind,
			"metadata":   map[string]interface{}{"name": meta.Name, "namespace": meta.Namespace},
		}}
		_, err = c.typeInfo.Interface(c.client, meta.Namespace).
			Apply(ctx, meta.Name, &empty, metav1.ApplyOptions{FieldManager: fieldManager})
		if err != nil {
			resp.Diagnostics.AddError("Unable to remove old fieldManager", err.Error())
			return
		}
	}

	var state types.KubernetesObjectValue
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("manifest"), &state)...)
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
	obj, err := c.typeInfo.Interface(c.client, meta.Namespace).
		Apply(ctx, meta.Name, obj, metav1.ApplyOptions{FieldManager: planFieldManager, Force: *forceConflicts})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update resource", err.Error())
		return
	}

	fields, diags := generic.GetManagedFieldSet(obj, planFieldManager)
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("field_manager"), planFieldManager)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force_conflicts"), forceConflicts)...)
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
	var expectedFormat string
	if c.typeInfo.Namespaced {
		expectedFormat = "fieldManager:namespace/name"
	} else {
		expectedFormat = "fieldManager:name"
	}

	components := strings.SplitN(req.ID, ":", 2)
	if len(components) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected %s import, got %s", expectedFormat, req.ID))
		return
	}

	fieldManager := components[0]
	resource := components[1]

	metadata := make(map[string]any)
	if c.typeInfo.Namespaced {
		components = strings.SplitN(resource, "/", 2)
		if len(components) != 2 {
			resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected %s resource, got %s", expectedFormat, resource))
			return
		}
		metadata["namespace"] = components[0]
		metadata["name"] = components[1]
	} else {
		metadata["name"] = resource
	}

	obj := unstructured.Unstructured{Object: map[string]any{"metadata": metadata}}
	var state types.KubernetesObjectValue
	resp.Diagnostics.Append(generic.UnstructuredToValue(ctx, c.typeInfo.Schema, obj, nil, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("manifest"), state)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("field_manager"), fieldManager)...)
}

func (c *crdResource) MoveState(ctx context.Context) []tfresource.StateMover {
	return []tfresource.StateMover{{
		SourceSchema: &schema.Schema{
			Version: 0,
			Attributes: map[string]schema.Attribute{
				"manifest": generic.OpenApiToTfSchema(ctx, c.typeInfo, false),
				"field_manager": schema.StringAttribute{
					Required: false,
					Computed: true,
					Optional: true,
					Default:  stringdefault.StaticString(defaultFieldManager),
				},
				"force_conflicts": schema.BoolAttribute{
					Required: false,
					Computed: true,
					Optional: true,
					Default:  booldefault.StaticBool(false),
				},
			},
		},
		StateMover: func(ctx context.Context, req tfresource.MoveStateRequest, resp *tfresource.MoveStateResponse) {
			if req.SourceTypeName != typeName("k8scrd", c.typeInfo) {
				return
			}
			// No-op, only the resource name has changed
			resp.TargetState = *req.SourceState
		},
	}}
}

var (
	_ tfresource.Resource                = &crdResource{}
	_ tfresource.ResourceWithConfigure   = &crdResource{}
	_ tfresource.ResourceWithImportState = &crdResource{}
	_ tfresource.ResourceWithMoveState   = &crdResource{}
)
