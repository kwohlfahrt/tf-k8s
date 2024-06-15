package crd_test

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/kwohlfahrt/terraform-provider-k8scrd/internal/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func checkExists(client *dynamic.DynamicClient, gvr runtimeschema.GroupVersionResource, meta types.ObjectMeta, exists bool) func(*terraform.State) error {
	return func(*terraform.State) error {
		_, err := client.Resource(gvr).Namespace(meta.Namespace).
			Get(context.TODO(), meta.Name, metav1.GetOptions{})

		if err != nil {
			if errors.IsGone(err) || errors.IsNotFound(err) {
				if exists {
					return fmt.Errorf("Resource %s does not exist", meta.Id())
				}
			} else {
				return err
			}
		} else {
			if !exists {
				return fmt.Errorf("Resource %s still exists", meta.Id())
			}
		}
		return nil
	}
}
