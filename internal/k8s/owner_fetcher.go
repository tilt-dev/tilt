package k8s

import (
	"fmt"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// The ObjectRefTree only contains immutable properties
// of a Kubernetes object: the name, namespace, and UID
type ObjectRefTree struct {
	Ref    v1.ObjectReference
	Owners []ObjectRefTree
}

func (t ObjectRefTree) UIDs() []types.UID {
	result := []types.UID{t.Ref.UID}
	for _, owner := range t.Owners {
		result = append(result, owner.UIDs()...)
	}
	return result
}

func (t ObjectRefTree) stringLines() []string {
	result := []string{fmt.Sprintf("%s:%s", t.Ref.Kind, t.Ref.Name)}
	for _, owner := range t.Owners {
		// indent each of the owners by two spaces
		branchLines := owner.stringLines()
		for _, branchLine := range branchLines {
			result = append(result, fmt.Sprintf("  %s", branchLine))
		}
	}
	return result
}

func (t ObjectRefTree) String() string {
	return strings.Join(t.stringLines(), "\n")
}

type OwnerFetcher struct {
	kCli  Client
	cache map[types.UID]ObjectRefTree
	mu    *sync.RWMutex
}

func ProvideOwnerFetcher(kCli Client) OwnerFetcher {
	return OwnerFetcher{
		kCli:  kCli,
		cache: make(map[types.UID]ObjectRefTree),
		mu:    &sync.RWMutex{},
	}
}

func (v OwnerFetcher) getCachedTree(id types.UID) (ObjectRefTree, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	tree, ok := v.cache[id]
	return tree, ok
}

func (v OwnerFetcher) setCachedTree(id types.UID, tree ObjectRefTree) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cache[id] = tree
}

func (v OwnerFetcher) OwnerTreeOf(entity K8sEntity) (ObjectRefTree, error) {
	meta := entity.meta()
	uid := meta.GetUID()
	if uid == "" {
		return ObjectRefTree{}, fmt.Errorf("Can only get owners of deployed entities")
	}

	tree, ok := v.getCachedTree(uid)
	if ok {
		return tree, nil
	}

	apiVersion, kind := entity.GVK().ToAPIVersionAndKind()
	ref := v1.ObjectReference{
		Kind:       kind,
		APIVersion: apiVersion,
		Name:       meta.GetName(),
		Namespace:  meta.GetNamespace(),
		UID:        meta.GetUID(),
	}
	tree = ObjectRefTree{Ref: ref}

	owners, err := v.ownersOfMeta(meta)
	if err != nil {
		return ObjectRefTree{}, err
	}
	for _, owner := range owners {
		ownerTree, err := v.OwnerTreeOf(owner)
		if err != nil {
			return ObjectRefTree{}, err
		}
		tree.Owners = append(tree.Owners, ownerTree)
	}
	v.setCachedTree(uid, tree)
	return tree, nil
}

func (v OwnerFetcher) ownersOfMeta(meta k8sMeta) ([]K8sEntity, error) {
	owners := meta.GetOwnerReferences()
	result := make([]K8sEntity, 0, len(owners))
	for _, owner := range owners {
		ref := v1.ObjectReference{
			APIVersion: owner.APIVersion,
			Kind:       owner.Kind,
			Namespace:  meta.GetNamespace(),
			Name:       owner.Name,
			UID:        owner.UID,
		}

		owner, err := v.kCli.GetByReference(ref)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		result = append(result, owner)
	}

	return result, nil
}
