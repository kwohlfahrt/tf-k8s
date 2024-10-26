package generic_test

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/generic"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/provider"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestExtract(t *testing.T) {
	kubeconfig, err := os.ReadFile(os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	dynamic, err := provider.MakeDynamicClient(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}

	fieldManager := "testing"
	name := "test"
	namespace := "default"
	resource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	spec := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    map[string]interface{}{"foo": "bar"},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test-image",
					},
				},
			},
		},
	}
	obj, err := dynamic.Resource(resource).Namespace(namespace).Apply(
		context.TODO(), name, spec, metav1.ApplyOptions{FieldManager: fieldManager},
	)

	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		dynamic.Resource(resource).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	})

	extracted, err := generic.Extract(obj, fieldManager)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !reflect.DeepEqual(spec, extracted) {
		t.Fatal("extracted object does not match object spec")
	}
}
