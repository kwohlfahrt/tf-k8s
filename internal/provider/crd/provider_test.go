package crd_test

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func checkExists(client *dynamic.DynamicClient, gvr metav1.GroupVersionResource, meta metav1.ObjectMeta) func(*terraform.State) error {
	return func(*terraform.State) error {
		schemaGvr := runtimeschema.GroupVersionResource{Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource}
		_, err := client.Resource(schemaGvr).Namespace(meta.Namespace).
			Get(context.TODO(), meta.Name, metav1.GetOptions{})

		if err != nil {
			return err
		}
		return nil
	}
}

func checkNotExists(client *dynamic.DynamicClient, gvr metav1.GroupVersionResource, meta metav1.ObjectMeta) func(*terraform.State) error {
	return func(*terraform.State) error {
		schemaGvr := runtimeschema.GroupVersionResource{Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource}
		obj, err := client.Resource(schemaGvr).Namespace(meta.Namespace).
			Get(context.TODO(), meta.Name, metav1.GetOptions{})

		var id string
		if meta.Namespace != "" {
			id = fmt.Sprintf("%s/%s", meta.Namespace, meta.Name)
		} else {
			id = meta.Name
		}

		if err != nil {
			if k8serrors.IsGone(err) || k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if obj.GetDeletionTimestamp() == nil {
			return fmt.Errorf("Resource '%s' still exists", id)
		}
		return nil
	}
}

type checkProperty struct {
	Name  string
	Path  []interface{}
	Value interface{}
}

type checkResource struct {
	GroupVersionResource metav1.GroupVersionResource
	Metadata             metav1.ObjectMeta
}

type checkSpec struct {
	Resources  []checkResource
	Properties []checkProperty
}

func parsePath(path []interface{}) tfjsonpath.Path {
	var out tfjsonpath.Path
	switch start := path[0].(type) {
	case string:
		out = tfjsonpath.New(start)
	case int:
		out = tfjsonpath.New(start)
	default:
		panic("Unsupported path type!")
	}

	for _, p := range path[1:] {
		switch p := p.(type) {
		case string:
			out = out.AtMapKey(p)
		case int:
			out = out.AtSliceIndex(p)
		case float64:
			out = out.AtSliceIndex(int(p))
		default:
			panic("unsupported path type")
		}
	}

	return out
}

func parseValue(value interface{}) knownvalue.Check {
	switch value := value.(type) {
	case string:
		return knownvalue.StringExact(value)
	case int64:
		return knownvalue.Int64Exact(value)
	case float64:
		return knownvalue.Float64Exact(value)
	default:
		panic("unsupported value type")
	}
}

func makeChecks(client *dynamic.DynamicClient, spec []checkResource) resource.TestCheckFunc {
	checks := make([]resource.TestCheckFunc, 0, len(spec))

	for _, resource := range spec {
		check := checkExists(client, resource.GroupVersionResource, resource.Metadata)
		checks = append(checks, check)
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)
}

func makeConfigChecks(spec []checkProperty) resource.ConfigPlanChecks {
	checks := make([]plancheck.PlanCheck, 0, len(spec))

	for _, property := range spec {
		check := plancheck.ExpectKnownValue(property.Name, parsePath(property.Path), parseValue(property.Value))
		checks = append(checks, check)
	}

	return resource.ConfigPlanChecks{PostApplyPostRefresh: checks}
}

func makeStateChecks(spec []checkProperty) []statecheck.StateCheck {
	checks := make([]statecheck.StateCheck, 0, len(spec))

	for _, property := range spec {
		check := statecheck.ExpectKnownValue(property.Name, parsePath(property.Path), parseValue(property.Value))
		checks = append(checks, check)
	}

	return checks
}

func makeDestroyChecks(client *dynamic.DynamicClient, resources []checkResource) resource.TestCheckFunc {
	checks := make([]resource.TestCheckFunc, 0, len(resources))

	for _, resource := range resources {
		check := checkNotExists(client, resource.GroupVersionResource, resource.Metadata)
		checks = append(checks, check)
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)
}
