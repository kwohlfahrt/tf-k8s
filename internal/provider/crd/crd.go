//go:generate openapi ./versions/example.com.yaml
package crd

import (
	"bytes"
	_ "embed"
	"encoding/gob"
	"errors"
	"io"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
)

//go:embed typeInfos.bin
var typeInfos []byte
var TypeInfos []generic.TypeInfo

func init() {
	gob.Register(types.KubernetesObjectType{})
	gob.Register(types.KubernetesListType{})
	gob.Register(types.KubernetesMapType{})
	gob.Register(types.KubernetesUnionType{})
	gob.Register(basetypes.BoolType{})
	gob.Register(basetypes.Int64Type{})
	gob.Register(basetypes.Float64Type{})
	gob.Register(basetypes.NumberType{})
	gob.Register(basetypes.StringType{})
	gob.Register(basetypes.StringType{})

	reader := bytes.NewReader(typeInfos)
	dec := gob.NewDecoder(reader)
	for {
		var info generic.TypeInfo
		err := dec.Decode(&info)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			panic(err)
		}
		TypeInfos = append(TypeInfos, info)
	}
}
