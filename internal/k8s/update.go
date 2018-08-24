package k8s

import "context"

// Update the given entities.
//
// Most entities can be updated with a plain Apply. But some k8s entities
// are immutable, and need to be deleted first.
func Update(ctx context.Context, client Client, entities []K8sEntity) error {
	toDelete := ImmutableEntities(entities)
	if len(toDelete) > 0 {
		err := client.Delete(ctx, toDelete)
		if err != nil {
			return err
		}
	}
	return client.Apply(ctx, entities)
}
