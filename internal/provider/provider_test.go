package provider

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func assertExists(client *dynamic.DynamicClient, namespace string, name string, exists bool) error {
	_, err := client.Resource(certificateGvr).Namespace(namespace).
		Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		if errors.IsGone(err) || errors.IsNotFound(err) {
			if exists {
				return fmt.Errorf("Resource %s/%s does not exist", namespace, name)
			}
		}
		return err
	} else {
		if !exists {
			return fmt.Errorf("Resource %s/%s still exists", namespace, name)
		}
	}
	return nil
}
