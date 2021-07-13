package indexer

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// List all the objects owned by the given object type.
//
// This allows us to use owner-based indexing in tests without
// incurring the overhead of caching.
//
// See discussion here:
// https://github.com/tilt-dev/tilt/issues/4719
func ListOwnedBy(ctx context.Context, client ctrlclient.Client, list ctrlclient.ObjectList, nn types.NamespacedName, ownerType metav1.TypeMeta) error {
	err := client.List(ctx, list, ctrlclient.InNamespace(nn.Namespace))
	if err != nil {
		return err
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}

	result := []runtime.Object{}
	for _, item := range items {
		clientObj, ok := item.(ctrlclient.Object)
		if !ok {
			continue
		}
		owner := metav1.GetControllerOf(clientObj)
		if owner == nil {
			continue
		}
		if owner.APIVersion != ownerType.APIVersion || owner.Kind != ownerType.Kind {
			continue
		}
		if owner.Name != nn.Name {
			continue
		}
		result = append(result, item)
	}
	return meta.SetList(list, result)
}
