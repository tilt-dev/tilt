package filepath

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
)

// ErrFileNotExists means the file doesn't actually exist.
var ErrFileNotExists = fmt.Errorf("file doesn't exist")

// ErrNamespaceNotExists means the directory for the namespace doesn't actually exist.
var ErrNamespaceNotExists = errors.New("namespace does not exist")

var _ rest.StandardStorage = &filepathREST{}
var _ rest.Scoper = &filepathREST{}
var _ rest.Storage = &filepathREST{}
var _ rest.ShortNamesProvider = &filepathREST{}
var _ rest.SingularNameProvider = &filepathREST{}

// NewFilepathREST instantiates a new REST storage.
func NewFilepathREST(
	fs FS,
	ws *WatchSet,
	strategy Strategy,
	groupResource schema.GroupResource,
	codec runtime.Codec,
	rootpath string,
	newFunc func() runtime.Object,
	newListFunc func() runtime.Object,
) rest.Storage {
	objRoot := filepath.Join(rootpath, groupResource.Group, groupResource.Resource)
	if err := fs.EnsureDir(objRoot); err != nil {
		panic(fmt.Sprintf("unable to write data dir: %s", err))
	}

	// file REST
	rest := &filepathREST{
		TableConvertor: rest.NewDefaultTableConvertor(groupResource),
		codec:          codec,
		objRootPath:    objRoot,
		newFunc:        newFunc,
		newListFunc:    newListFunc,
		strategy:       strategy,
		groupResource:  groupResource,
		fs:             fs,
		watchSet:       ws,
	}
	return rest
}

type filepathREST struct {
	rest.TableConvertor
	codec       runtime.Codec
	objRootPath string

	newFunc     func() runtime.Object
	newListFunc func() runtime.Object

	strategy      Strategy
	groupResource schema.GroupResource
	fs            FS
	watchSet      *WatchSet
}

func (f *filepathREST) notifyWatchers(ev watch.Event) {
	f.watchSet.notifyWatchers(ev)
}

func (f *filepathREST) New() runtime.Object {
	return f.newFunc()
}

func (f *filepathREST) NewList() runtime.Object {
	return f.newListFunc()
}

func (f *filepathREST) Destroy() {
	// Destroy() is intended for cleaning up client connections. Do nothing.
}

func (f *filepathREST) NamespaceScoped() bool {
	return f.strategy.NamespaceScoped()
}

func (f *filepathREST) ShortNames() []string {
	return f.strategy.ShortNames()
}

func (f *filepathREST) GetSingularName() string {
	return f.strategy.GetSingularName()
}

func (f *filepathREST) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	obj, err := f.fs.Read(f.codec, f.objectFileName(ctx, name), f.newFunc)
	if err != nil && os.IsNotExist(err) {
		return nil, apierrors.NewNotFound(f.groupResource, name)
	}
	return obj, err
}

func (f *filepathREST) List(
	ctx context.Context,
	options *metainternalversion.ListOptions,
) (runtime.Object, error) {
	p := newSelectionPredicate(options)
	newListObj := f.NewList()
	v, err := getListPrt(newListObj)
	if err != nil {
		return nil, err
	}

	dirname := f.objectDirName(ctx)
	rev, err := f.fs.VisitDir(dirname, f.newFunc, f.codec, func(path string, obj runtime.Object) error {
		ok, err := p.Matches(obj)
		if err != nil {
			return err
		}
		if ok {
			appendItem(v, obj)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed walking filepath %v: %v", dirname, err)
	}

	err = setResourceVersion(newListObj, rev)
	if err != nil {
		return nil, err
	}
	return newListObj, nil
}

func (f *filepathREST) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	rest.FillObjectMetaSystemFields(accessor)

	if err := rest.BeforeCreate(f.strategy, ctx, obj); err != nil {
		return nil, err
	}

	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	if f.NamespaceScoped() {
		// ensures namespace dir
		ns, ok := genericapirequest.NamespaceFrom(ctx)
		if !ok {
			return nil, ErrNamespaceNotExists
		}
		if err := f.fs.EnsureDir(filepath.Join(f.objRootPath, ns)); err != nil {
			return nil, err
		}
	}

	filename := f.objectFileName(ctx, accessor.GetName())

	if f.fs.Exists(filename) {
		return nil, apierrors.NewAlreadyExists(f.groupResource, accessor.GetName())
	}

	if err := f.fs.Write(f.codec, filename, obj, 0); err != nil {
		if errors.Is(err, VersionError) {
			err = f.conflictErr(accessor.GetName())
		}
		return nil, err
	}

	f.notifyWatchers(watch.Event{
		Type:   watch.Added,
		Object: obj,
	})

	return obj, nil
}

func (f *filepathREST) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	var isCreate bool
	var isDelete bool
	// attempt to update the object, automatically retrying on storage-level conflicts
	// (see guaranteedUpdate docs for details)
	obj, err := f.guaranteedUpdate(ctx, name, func(input runtime.Object) (output runtime.Object, err error) {
		isCreate = false
		isDelete = false

		if input == nil {
			if !forceAllowCreate {
				return nil, apierrors.NewNotFound(f.groupResource, name)
			}
			isCreate = true
		}
		inputVersion, err := getResourceVersion(input)
		if err != nil {
			return nil, err
		}

		// TODO: should not be necessary, verify Get works before creating filepath
		if f.NamespaceScoped() {
			// ensures namespace dir
			ns, ok := genericapirequest.NamespaceFrom(ctx)
			if !ok {
				return nil, ErrNamespaceNotExists
			}
			if err := f.fs.EnsureDir(filepath.Join(f.objRootPath, ns)); err != nil {
				return nil, err
			}
		}

		output, err = objInfo.UpdatedObject(ctx, input)
		if err != nil {
			return nil, err
		}

		// this check MUST happen before rest.BeforeUpdate is called - for subresource updates, it'll
		// use the input (storage version) to copy the subresource to (to avoid changing the spec),
		// so the version from the request object will be lost, breaking optimistic concurrency
		updatedVersion, err := getResourceVersion(output)
		if err != nil {
			return nil, err
		}
		if inputVersion != updatedVersion {
			return nil, f.conflictErr(name)
		}

		if err := rest.BeforeUpdate(f.strategy, ctx, output, input); err != nil {
			return nil, err
		}

		if isCreate {
			if createValidation != nil {
				if err := createValidation(ctx, input); err != nil {
					return nil, err
				}
			}
			return output, nil
		}

		if updateValidation != nil {
			if err := updateValidation(ctx, output, input); err != nil {
				return nil, err
			}
		}

		outputMeta, err := meta.Accessor(output)
		if err != nil {
			return nil, err
		}

		// handle 2-phase deletes -> for entities with finalizers, DeletionTimestamp is set and reconcilers execute +
		// remove them (triggering more updates); once drained, it can be deleted from the final update operation
		// loosely based off https://github.com/kubernetes/apiserver/blob/947ebe755ed8aed2e0f0f5d6420caad07fc04cc2/pkg/registry/generic/registry/store.go#L624
		if len(outputMeta.GetFinalizers()) == 0 && !outputMeta.GetDeletionTimestamp().IsZero() {
			// to simplify semantics here, we allow this update to go through and then
			// delete it - if this becomes a bottleneck (seems unlikely), we can delete
			// here and return a special sentinel error
			isDelete = true
			return output, nil
		}

		return output, nil
	})
	if err != nil {
		// TODO(milas): we need a better way of handling standard errors and
		// 	wrapping any others in generic apierrors - returning plain Go errors
		// 	(which still happens in some code paths) makes apiserver log out
		// 	warnings, though it doesn't actually break things so is not critical
		if os.IsNotExist(err) {
			return nil, false, apierrors.NewNotFound(f.groupResource, name)
		}
		return nil, false, err
	}

	if isCreate {
		f.notifyWatchers(watch.Event{
			Type:   watch.Added,
			Object: obj,
		})
		return obj, true, nil
	}

	if isDelete {
		filename := f.objectFileName(ctx, name)
		if err := f.fs.Remove(filename); err != nil {
			if os.IsNotExist(err) {
				return nil, false, apierrors.NewNotFound(f.groupResource, name)
			}
			return nil, false, err
		}
		f.notifyWatchers(watch.Event{
			Type:   watch.Deleted,
			Object: obj,
		})
		return obj, false, nil
	}

	f.notifyWatchers(watch.Event{
		Type:   watch.Modified,
		Object: obj,
	})
	return obj, false, nil
}

func (f *filepathREST) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	filename := f.objectFileName(ctx, name)
	oldObj, err := f.Get(ctx, name, nil)
	if err != nil {
		return nil, false, err
	}
	if deleteValidation != nil {
		if err := deleteValidation(ctx, oldObj); err != nil {
			return nil, false, err
		}
	}

	objMeta, err := meta.Accessor(oldObj)
	if err != nil {
		return nil, false, err
	}
	// loosely adapted from https://github.com/kubernetes/apiserver/blob/947ebe755ed8aed2e0f0f5d6420caad07fc04cc2/pkg/registry/generic/registry/store.go#L854-L877
	if len(objMeta.GetFinalizers()) != 0 {
		now := metav1.NewTime(time.Now())
		// per-contract, deletion timestamps can not be unset and can only be moved _earlier_
		if objMeta.GetDeletionTimestamp() == nil || now.Before(objMeta.GetDeletionTimestamp()) {
			objMeta.SetDeletionTimestamp(&now)
		}
		zero := int64(0)
		objMeta.SetDeletionGracePeriodSeconds(&zero)

		version, err := getResourceVersion(oldObj)
		if err != nil {
			return nil, false, err
		}

		if err := f.fs.Write(f.codec, filename, oldObj, version); err != nil {
			if errors.Is(err, VersionError) {
				err = f.conflictErr(name)
			}
			return nil, false, err
		}

		f.notifyWatchers(watch.Event{
			Type:   watch.Modified,
			Object: oldObj,
		})

		// false in return indicates object will be deleted asynchronously
		return oldObj, false, nil
	}

	if err := f.fs.Remove(filename); err != nil {
		if err != nil && os.IsNotExist(err) {
			return nil, false, apierrors.NewNotFound(f.groupResource, name)
		}
		return nil, false, err
	}
	f.notifyWatchers(watch.Event{
		Type:   watch.Deleted,
		Object: oldObj,
	})
	return oldObj, true, nil
}

func (f *filepathREST) DeleteCollection(
	ctx context.Context,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
	listOptions *metainternalversion.ListOptions,
) (runtime.Object, error) {
	p := newSelectionPredicate(listOptions)
	newListObj := f.NewList()
	v, err := getListPrt(newListObj)
	if err != nil {
		return nil, err
	}
	dirname := f.objectDirName(ctx)
	rev, err := f.fs.VisitDir(dirname, f.newFunc, f.codec, func(path string, obj runtime.Object) error {
		ok, err := p.Matches(obj)
		if err != nil {
			return err
		}
		if ok {
			_ = f.fs.Remove(path)
			appendItem(v, obj)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed walking filepath %v: %v", dirname, err)
	}

	err = setResourceVersion(newListObj, rev)
	if err != nil {
		return nil, err
	}
	return newListObj, nil
}

func (f *filepathREST) objectFileName(ctx context.Context, name string) string {
	if f.NamespaceScoped() {
		// FIXME: return error if namespace is not found
		ns, _ := genericapirequest.NamespaceFrom(ctx)
		return filepath.Join(f.objRootPath, ns, name+".json")
	}
	return filepath.Join(f.objRootPath, name+".json")
}

func (f *filepathREST) objectDirName(ctx context.Context) string {
	if f.NamespaceScoped() {
		// FIXME: return error if namespace is not found
		ns, _ := genericapirequest.NamespaceFrom(ctx)
		return filepath.Join(f.objRootPath, ns)
	}
	return filepath.Join(f.objRootPath)
}

// updateFunc should return the updated object to persist to storage.
//
// This function might be called more than once, so must be idempotent. If an
// error is returned from it, the error will be propagated and the update halted.
type updateFunc func(input runtime.Object) (output runtime.Object, err error)

// guaranteedUpdate keeps calling tryUpdate to update an object retrying the update
// until success if there is a storage-level conflict.
//
// The input object passed to tryUpdate may change across invocations of tryUpdate
// if other writers are simultaneously updating it, so tryUpdate needs to take into
// account the current contents of the object when deciding how the update object
// should look.
//
// The "guaranteed" in the name comes from a method of the same name in the
// Kubernetes apiserver/etcd code. Most of this method comment is copied from
// its godoc.
//
// See https://github.com/kubernetes/apiserver/blob/544b6014f353b0f5e7c6fd2d3e04a7810d0ba5fc/pkg/storage/interfaces.go#L205-L238
func (f *filepathREST) guaranteedUpdate(ctx context.Context, name string, tryUpdate updateFunc) (runtime.Object, error) {
	// technically, this loop should be safe to run indefinitely, but a cap is
	// applied to avoid bugs resulting in an infinite* loop
	//
	// if the cap is hit, an internal server error will be returned
	//
	// * really until the context is canceled, but busy looping here for ~30 secs
	//   until it times out is not great either
	const maxAttempts = 100
	for i := 0; i < maxAttempts; i++ {
		if err := ctx.Err(); err != nil {
			// the FS layer doesn't use context, so we explicitly check it on
			// each loop iteration so that we'll stop retrying if the context
			// gets canceled (e.g. request timeout)
			return nil, err
		}

		storageObj, err := f.Get(ctx, name, nil)
		if err != nil && !apierrors.IsNotFound(err) {
			// some objects allow create-on-update semantics, so NotFound is not terminal
			return nil, err
		}
		storageVersion, err := getResourceVersion(storageObj)
		if err != nil {
			return nil, err
		}

		out, err := tryUpdate(storageObj)
		if err != nil {
			// TODO(milas): check error type and wrap if necessary
			return nil, err
		}

		filename := f.objectFileName(ctx, name)
		if err := f.fs.Write(f.codec, filename, out, storageVersion); err != nil {
			if errors.Is(err, VersionError) {
				// storage conflict, retry
				continue
			}
			return nil, err
		}
		return out, nil
	}

	// a non-early return means the loop exhausted all attempts
	return nil, apierrors.NewInternalError(errors.New("failed to persist to storage"))
}

func appendItem(v reflect.Value, obj runtime.Object) {
	v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
}

func getListPrt(listObj runtime.Object) (reflect.Value, error) {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return reflect.Value{}, err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("need ptr to slice: %v", err)
	}
	return v, nil
}

func (f *filepathREST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	p := newSelectionPredicate(options)
	jw := f.watchSet.newWatch()

	getInitEvents := func() ([]watch.Event, error) {
		// On initial watch, send all the existing objects.
		// We may receive duplicated "Added" events for some objects via the watch updata channel,
		// and we may report them twice, but that is much better than not reporting them at all,
		// and having clients left unaware that an object they might be interested in was created.
		// See https://github.com/tilt-dev/tilt-apiserver/issues/88

		list, err := f.List(ctx, options)
		if err != nil {
			return nil, err
		}
		danger := reflect.ValueOf(list).Elem()
		items := danger.FieldByName("Items")

		initEvents := []watch.Event{}
		for i := 0; i < items.Len(); i++ {
			obj := items.Index(i).Addr().Interface().(runtime.Object)
			ok, err := p.Matches(obj)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			initEvents = append(initEvents, watch.Event{
				Type:   watch.Added,
				Object: obj,
			})
		}
		return initEvents, nil
	}

	startErr := jw.Start(p, getInitEvents)
	return jw, startErr
}

func (f *filepathREST) conflictErr(name string) error {
	return apierrors.NewConflict(
		f.groupResource,
		name,
		errors.New(registry.OptimisticLockErrorMsg))
}

func newSelectionPredicate(options *metainternalversion.ListOptions) storage.SelectionPredicate {
	p := storage.SelectionPredicate{
		Label:    labels.Everything(),
		Field:    fields.Everything(),
		GetAttrs: storage.DefaultClusterScopedAttr,
	}
	if options != nil {
		if options.LabelSelector != nil {
			p.Label = options.LabelSelector
		}
		if options.FieldSelector != nil {
			p.Field = options.FieldSelector
		}
	}
	return p
}
