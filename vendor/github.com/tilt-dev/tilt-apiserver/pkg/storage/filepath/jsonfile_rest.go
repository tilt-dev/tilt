package filepath

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

// ErrFileNotExists means the file doesn't actually exist.
var ErrFileNotExists = fmt.Errorf("file doesn't exist")

// ErrNamespaceNotExists means the directory for the namespace doesn't actually exist.
var ErrNamespaceNotExists = errors.New("namespace does not exist")

var _ rest.StandardStorage = &filepathREST{}
var _ rest.Scoper = &filepathREST{}
var _ rest.Storage = &filepathREST{}

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

	strategy       Strategy
	groupResource  schema.GroupResource
	fs             FS
	watchSet       *WatchSet
	currentVersion int64
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

func (f *filepathREST) NamespaceScoped() bool {
	return f.strategy.NamespaceScoped()
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
	newListObj := f.NewList()
	v, err := getListPrt(newListObj)
	if err != nil {
		return nil, err
	}

	dirname := f.objectDirName(ctx)
	if err := f.fs.VisitDir(dirname, f.newFunc, f.codec, func(path string, obj runtime.Object) {
		appendItem(v, obj)
	}); err != nil {
		return nil, fmt.Errorf("failed walking filepath %v: %v", dirname, err)
	}
	return newListObj, nil
}

func (f *filepathREST) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
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

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	filename := f.objectFileName(ctx, accessor.GetName())
	accessor.SetResourceVersion(fmt.Sprintf("%d", atomic.AddInt64(&f.currentVersion, 1)))

	if f.fs.Exists(filename) {
		return nil, apierrors.NewAlreadyExists(f.groupResource, accessor.GetName())
	}

	if err := f.fs.Write(f.codec, filename, obj); err != nil {
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
	isCreate := false
	oldObj, err := f.Get(ctx, name, nil)
	if err != nil {
		if !forceAllowCreate {
			return nil, false, err
		}
		isCreate = true
	}

	// TODO: should not be necessary, verify Get works before creating filepath
	if f.NamespaceScoped() {
		// ensures namespace dir
		ns, ok := genericapirequest.NamespaceFrom(ctx)
		if !ok {
			return nil, false, ErrNamespaceNotExists
		}
		if err := f.fs.EnsureDir(filepath.Join(f.objRootPath, ns)); err != nil {
			return nil, false, err
		}
	}

	updatedObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	if err := rest.BeforeUpdate(f.strategy, ctx, updatedObj, oldObj); err != nil {
		return nil, false, err
	}

	filename := f.objectFileName(ctx, name)

	if isCreate {
		if createValidation != nil {
			if err := createValidation(ctx, updatedObj); err != nil {
				return nil, false, err
			}
		}
		if err := f.fs.Write(f.codec, filename, updatedObj); err != nil {
			return nil, false, err
		}
		f.notifyWatchers(watch.Event{
			Type:   watch.Added,
			Object: updatedObj,
		})
		return updatedObj, true, nil
	}

	if updateValidation != nil {
		if err := updateValidation(ctx, updatedObj, oldObj); err != nil {
			return nil, false, err
		}
	}

	objMeta, err := meta.Accessor(updatedObj)
	if err != nil {
		return nil, false, err
	}
	objMeta.SetResourceVersion(fmt.Sprintf("%d", atomic.AddInt64(&f.currentVersion, 1)))

	// handle 2-phase deletes -> for entities with finalizers, DeletionTimestamp is set and reconcilers execute +
	// remove them (triggering more updates); once drained, it can be deleted from the final update operation
	// loosely based off https://github.com/kubernetes/apiserver/blob/947ebe755ed8aed2e0f0f5d6420caad07fc04cc2/pkg/registry/generic/registry/store.go#L624
	if len(objMeta.GetFinalizers()) == 0 && !objMeta.GetDeletionTimestamp().IsZero() {
		if err := f.fs.Remove(filename); err != nil {
			return nil, false, err
		}
		f.notifyWatchers(watch.Event{
			Type:   watch.Deleted,
			Object: updatedObj,
		})
		return updatedObj, false, nil
	}

	if err := f.fs.Write(f.codec, filename, updatedObj); err != nil {
		return nil, false, err
	}
	f.notifyWatchers(watch.Event{
		Type:   watch.Modified,
		Object: updatedObj,
	})
	return updatedObj, false, nil
}

func (f *filepathREST) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	filename := f.objectFileName(ctx, name)
	if !f.fs.Exists(filename) {
		return nil, false, ErrFileNotExists
	}

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

		if err := f.fs.Write(f.codec, filename, oldObj); err != nil {
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
	newListObj := f.NewList()
	v, err := getListPrt(newListObj)
	if err != nil {
		return nil, err
	}
	dirname := f.objectDirName(ctx)
	if err := f.fs.VisitDir(dirname, f.newFunc, f.codec, func(path string, obj runtime.Object) {
		_ = f.fs.Remove(path)
		appendItem(v, obj)
	}); err != nil {
		return nil, fmt.Errorf("failed walking filepath %v: %v", dirname, err)
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
	jw := f.watchSet.newWatch()

	// On initial watch, send all the existing objects
	list, err := f.List(ctx, options)
	if err != nil {
		return nil, err
	}

	danger := reflect.ValueOf(list).Elem()
	items := danger.FieldByName("Items")

	for i := 0; i < items.Len(); i++ {
		obj := items.Index(i).Addr().Interface().(runtime.Object)
		jw.ch <- watch.Event{
			Type:   watch.Added,
			Object: obj,
		}
	}

	f.watchSet.start(jw)

	return jw, nil
}
