package types

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// CRD schemas just define metadata as "object", with no more detail. Hard-code it here.
var MetadataType KubernetesObjectType = KubernetesObjectType{
	ObjectType: basetypes.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":      basetypes.StringType{},
			"namespace": basetypes.StringType{},
		},
	},
	FieldNames:     map[string]string{"name": "name", "namespace": "namespace"},
	RequiredFields: map[string]bool{"name": true, "namespace": true},
}

type ObjectMeta struct {
	Name      string `tfsdk:"name"`
	Namespace string `tfsdk:"namespace"`
}
