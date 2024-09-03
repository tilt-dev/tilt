// Package filepath provides filepath storage related utilities.
package filepath

import (
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
	builderrest "github.com/tilt-dev/tilt-apiserver/pkg/server/builder/rest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
)

// NewJSONFilepathStorageProvider use local host path as persistent layer storage:
//
//   - For namespaced-scoped resources: the resource will be written under the root-path in
//     the following structure:
//
//     -- (root-path) --- /namespace1/ --- resource1
//     |                |
//     |                --- resource2
//     |
//     --- /namespace2/ --- resource3
//
//   - For cluster-scoped resources, there will be no mid-layer folders for namespaces:
//
//     -- (root-path) --- resource1
//     |
//     --- resource2
//     |
//     --- resource3
//
// Args:
//
// fs: An abstraction over the filesystem, so that the JSON can be stored in memory or on-disk.
// watchSet: Storage for watchers to be notified of this resource type. Each type should have its own
//
//	WatchSet, but subresources (like the status subresource) should share a WatchSet with their parent.
func NewJSONFilepathStorageProvider(obj resource.Object, rootPath string, fs FS, watchSet *WatchSet, strategy Strategy) builderrest.ResourceHandlerProvider {
	return func(scheme *runtime.Scheme, getter generic.RESTOptionsGetter) (rest.Storage, error) {
		gr := obj.GetGroupVersionResource().GroupResource()
		opt, err := getter.GetRESTOptions(gr, obj)
		if err != nil {
			return nil, err
		}
		codec := opt.StorageConfig.Codec
		return NewFilepathREST(
			fs,
			watchSet,
			strategy,
			gr,
			codec,
			rootPath,
			obj.New,
			obj.NewList,
		), nil
	}
}
