package crd_test

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func checkExists(client *dynamic.DynamicClient, gvr metav1.GroupVersionResource, meta metav1.ObjectMeta, exists bool) func(*terraform.State) error {
	return func(*terraform.State) error {
		schemaGvr := runtimeschema.GroupVersionResource{Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource}
		_, err := client.Resource(schemaGvr).Namespace(meta.Namespace).
			Get(context.TODO(), meta.Name, metav1.GetOptions{})

		var id string
		if meta.Namespace != "" {
			id = fmt.Sprintf("%s/%s", meta.Namespace, meta.Name)
		} else {
			id = meta.Name
		}

		if err != nil {
			if k8serrors.IsGone(err) || k8serrors.IsNotFound(err) {
				if exists {
					return fmt.Errorf("Resource '%s' does not exist", id)
				}
			} else {
				return err
			}
		} else {
			if !exists {
				return fmt.Errorf("Resource '%s' still exists", id)
			}
		}
		return nil
	}
}

type checkProperty struct {
	Name  string
	Path  string
	Value string
}

type checkResource struct {
	GroupVersionResource metav1.GroupVersionResource
	Metadata             metav1.ObjectMeta
}

type checkSpec struct {
	Resources  []checkResource
	Properties []checkProperty
}

func makeChecks(client *dynamic.DynamicClient, spec checkSpec) resource.TestCheckFunc {
	checks := make([]resource.TestCheckFunc, 0, len(spec.Resources)+len(spec.Properties))

	for _, resource := range spec.Resources {
		check := checkExists(client, resource.GroupVersionResource, resource.Metadata, true)
		checks = append(checks, check)
	}
	for _, property := range spec.Properties {
		check := resource.TestCheckResourceAttr(property.Name, property.Path, property.Value)
		checks = append(checks, check)
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func makeDestroyChecks(client *dynamic.DynamicClient, resources []checkResource) resource.TestCheckFunc {
	checks := make([]resource.TestCheckFunc, 0, len(resources))

	for _, resource := range resources {
		check := checkExists(client, resource.GroupVersionResource, resource.Metadata, false)
		checks = append(checks, check)
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)
}
