package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"

	"github.com/tilt-dev/tilt/pkg/logger"
)

var PodGVR = v1.SchemeGroupVersion.WithResource("pods")
var ServiceGVR = v1.SchemeGroupVersion.WithResource("services")
var EventGVR = v1.SchemeGroupVersion.WithResource("events")

// Inspired by:
// https://groups.google.com/g/kubernetes-sig-api-machinery/c/PbSCXdLDno0/m/v9gH3HXVDAAJ
const resyncPeriod = 15 * time.Minute

// A wrapper object around SharedInformer objects, to make them
// a bit easier to use correctly.
type ObjectUpdate struct {
	obj      interface{}
	isDelete bool
}

// Returns a Pod if this is a pod Add or a pod Update.
func (r ObjectUpdate) AsPod() (*v1.Pod, bool) {
	if r.isDelete {
		return nil, false
	}
	pod, ok := r.obj.(*v1.Pod)
	return pod, ok
}

// Returns (namespace, name, isDelete).
//
// The informer's OnDelete handler sometimes gives us a structured object, and
// sometimes returns a DeletedFinalStateUnknown object. To make this easier to
// handle correctly, we never allow access to the OnDelete object. Instead, we
// force the caller to use AsDeletedKey() to get the identifier of the object.
//
// For more info, see:
// https://godoc.org/k8s.io/client-go/tools/cache#ResourceEventHandler
func (r ObjectUpdate) AsDeletedKey() (Namespace, string, bool) {
	if !r.isDelete {
		return "", "", false
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(r.obj)
	if err != nil {
		return "", "", false
	}
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return "", "", false
	}
	return Namespace(ns), name, true
}

func maybeUnpackStatusError(err error) error {
	statusErr, isStatusErr := err.(*apiErrors.StatusError)
	if !isStatusErr {
		return err
	}
	status := statusErr.ErrStatus
	return fmt.Errorf("%s, Reason: %s, Code: %d", status.Message, status.Reason, status.Code)
}

func (kCli K8sClient) makeInformer(
	ctx context.Context,
	ns Namespace,
	gvr schema.GroupVersionResource) (cache.SharedInformer, error) {
	if ns == "" {
		return nil, fmt.Errorf("makeInformer no longer supports watching all namespaces")
	}

	// HACK(dmiller): There's no way to get errors out of an informer. See https://github.com/kubernetes/client-go/issues/155
	// In the meantime, at least to get authorization and some other errors let's try to set up a watcher and then just
	// throw it away.
	watcher, err := kCli.dynamic.Resource(gvr).Namespace(ns.String()).
		Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(maybeUnpackStatusError(err), "makeInformer")
	}
	watcher.Stop()

	options := []informers.SharedInformerOption{
		informers.WithNamespace(ns.String()),
	}

	factory := informers.NewSharedInformerFactoryWithOptions(kCli.clientset, resyncPeriod, options...)
	resFactory, err := factory.ForResource(gvr)
	if err != nil {
		return nil, errors.Wrap(err, "makeInformer")
	}

	return resFactory.Informer(), nil
}

func (kCli K8sClient) WatchEvents(ctx context.Context, ns Namespace) (<-chan *v1.Event, error) {
	gvr := EventGVR
	informer, err := kCli.makeInformer(ctx, ns, gvr)
	if err != nil {
		return nil, errors.Wrap(err, "WatchEvents")
	}

	ch := make(chan *v1.Event)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj, ok := obj.(*v1.Event)
			if ok {
				ch <- mObj
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			mObj, ok := newObj.(*v1.Event)
			if ok {
				oldObj, ok := oldObj.(*v1.Event)
				// the informer regularly gives us updates for events where cmp.Equal(oldObj, newObj) returns true.
				// we have not investigated why it does this, but these updates seem to always be spurious and
				// uninteresting.
				// we could check cmp.Equal here, but really, `Count` is probably the only reason we even care about
				// updates at all.
				if !ok || oldObj.Count < mObj.Count {
					ch <- mObj
				}
			}
		},
	})

	go runInformer(ctx, "events", informer)

	return ch, nil
}

func (kCli K8sClient) WatchPods(ctx context.Context, ns Namespace) (<-chan ObjectUpdate, error) {
	gvr := PodGVR
	informer, err := kCli.makeInformer(ctx, ns, gvr)
	if err != nil {
		return nil, errors.Wrap(err, "WatchPods")
	}

	ch := make(chan ObjectUpdate)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj, ok := obj.(*v1.Pod)
			if ok {
				obj = FixContainerStatusImagesNoMutation(mObj)
			}
			ch <- ObjectUpdate{obj: obj}
		},
		DeleteFunc: func(obj interface{}) {
			mObj, ok := obj.(*v1.Pod)
			if ok {
				obj = FixContainerStatusImagesNoMutation(mObj)
			}
			ch <- ObjectUpdate{obj: obj, isDelete: true}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			oldPod, ok := oldObj.(*v1.Pod)
			if !ok {
				return
			}

			newPod, ok := newObj.(*v1.Pod)
			if !ok || oldPod == newPod {
				return
			}

			newPod = FixContainerStatusImagesNoMutation(newPod)
			ch <- ObjectUpdate{obj: newPod}
		},
	})

	go runInformer(ctx, "pods", informer)

	return ch, nil
}

func (kCli K8sClient) WatchServices(ctx context.Context, ns Namespace) (<-chan *v1.Service, error) {
	gvr := ServiceGVR
	informer, err := kCli.makeInformer(ctx, ns, gvr)
	if err != nil {
		return nil, errors.Wrap(err, "WatchServices")
	}

	ch := make(chan *v1.Service)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj, ok := obj.(*v1.Service)
			if ok {
				ch <- mObj
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			newService, ok := newObj.(*v1.Service)
			if ok {
				ch <- newService
			}
		},
	})

	go runInformer(ctx, "services", informer)

	return ch, nil
}

func supportsPartialMetadata(v *version.Info) bool {
	k1dot15, err := semver.ParseTolerant("v1.15.0")
	if err != nil {
		return false
	}
	version, err := semver.ParseTolerant(v.GitVersion)
	if err != nil {
		// If we don't recognize the version number,
		// assume this server doesn't support metadata.
		return false
	}
	return version.GTE(k1dot15)
}

func (kCli K8sClient) WatchMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) (<-chan ObjectMeta, error) {
	gvr, err := kCli.gvr(ctx, gvk)
	if err != nil {
		return nil, errors.Wrap(err, "WatchMeta")
	}

	version, err := kCli.discovery.ServerVersion()
	if err != nil {
		return nil, errors.Wrap(err, "WatchMeta")
	}

	if supportsPartialMetadata(version) {
		return kCli.watchMeta15Plus(ctx, gvr, ns)
	}
	return kCli.watchMeta14Minus(ctx, gvr, ns)
}

// workaround a bug in client-go
// https://github.com/kubernetes/client-go/issues/882
func (kCli K8sClient) watchMeta14Minus(ctx context.Context, gvr schema.GroupVersionResource, ns Namespace) (<-chan ObjectMeta, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(kCli.clientset, resyncPeriod, informers.WithNamespace(ns.String()))
	resFactory, err := factory.ForResource(gvr)
	if err != nil {
		return nil, errors.Wrap(err, "watchMeta")
	}
	informer := resFactory.Informer()
	ch := make(chan ObjectMeta)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj, ok := obj.(runtime.Object)
			if !ok {
				return
			}

			entity := NewK8sEntity(mObj)
			ch <- entity.meta()
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			mNewObj, ok := newObj.(runtime.Object)
			if !ok {
				return
			}

			entity := NewK8sEntity(mNewObj)
			ch <- entity.meta()
		},
	})

	go runInformer(ctx, fmt.Sprintf("%s-metadata", gvr.Resource), informer)

	return ch, nil
}

func (kCli K8sClient) watchMeta15Plus(ctx context.Context, gvr schema.GroupVersionResource, ns Namespace) (<-chan ObjectMeta, error) {
	factory := metadatainformer.NewFilteredSharedInformerFactory(kCli.metadata, resyncPeriod, ns.String(), func(*metav1.ListOptions) {})
	informer := factory.ForResource(gvr).Informer()

	ch := make(chan ObjectMeta)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj, ok := obj.(*metav1.PartialObjectMetadata)
			if ok {
				ch <- &mObj.ObjectMeta
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			mNewObj, ok := newObj.(*metav1.PartialObjectMetadata)
			if ok {
				ch <- &mNewObj.ObjectMeta
			}
		},
	})

	go runInformer(ctx, fmt.Sprintf("%s-metadata", gvr.Resource), informer)

	return ch, nil
}

func runInformer(ctx context.Context, name string, informer cache.SharedInformer) {
	originalDuration := 3 * time.Second
	originalBackoff := wait.Backoff{
		Steps:    1000,
		Duration: originalDuration,
		Factor:   3.0,
		Jitter:   0.5,
		Cap:      time.Hour,
	}
	backoff := originalBackoff
	lastErrorHandlerFinish := time.Time{}
	_ = informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		sleepTime := originalDuration
		if time.Since(lastErrorHandlerFinish) < time.Second {
			sleepTime = backoff.Step()
			logger.Get(ctx).Warnf("Pausing k8s %s watcher for %s: %v",
				name,
				sleepTime.Truncate(time.Second),
				err)
		} else {
			backoff = originalBackoff
		}

		select {
		case <-ctx.Done():
		case <-time.After(sleepTime):
		}
		lastErrorHandlerFinish = time.Now()
	})
	informer.Run(ctx.Done())
}
