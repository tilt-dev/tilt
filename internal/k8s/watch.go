package k8s

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/windmilleng/tilt/pkg/model"
)

type watcherFactory func(namespace string) watcher
type watcher interface {
	Watch(options metav1.ListOptions) (watch.Interface, error)
}

func (kCli K8sClient) makeWatcher(f watcherFactory, ls labels.Selector) (watch.Interface, Namespace, error) {
	// passing "" gets us all namespaces
	w := f("")
	if w == nil {
		return nil, "", nil
	}

	watcher, err := w.Watch(metav1.ListOptions{LabelSelector: ls.String()})
	if err == nil {
		return watcher, "", nil
	}

	// If the request failed, we might be able to recover.
	statusErr, isStatusErr := err.(*apiErrors.StatusError)
	if !isStatusErr {
		return nil, "", err
	}

	status := statusErr.ErrStatus
	if status.Code == http.StatusForbidden {
		// If this is a forbidden error, maybe the user just isn't allowed to watch this namespace.
		// Let's narrow our request to just the config namespace, and see if that helps.
		w := f(kCli.configNamespace.String())
		if w == nil {
			return nil, "", nil
		}

		watcher, err := w.Watch(metav1.ListOptions{LabelSelector: ls.String()})
		if err == nil {
			return watcher, kCli.configNamespace, nil
		}

		// ugh, it still failed. return the original error.
	}
	return nil, "", fmt.Errorf("%s, Reason: %s, Code: %d", status.Message, status.Reason, status.Code)
}

func (kCli K8sClient) makeInformer(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	ls labels.Selector) (cache.SharedInformer, error) {
	// HACK(dmiller): There's no way to get errors out of an informer. See https://github.com/kubernetes/client-go/issues/155
	// In the meantime, at least to get authorization and some other errors let's try to set up a watcher and then just
	// throw it away.
	watcher, ns, err := kCli.makeWatcher(func(ns string) watcher {
		return kCli.dynamic.Resource(gvr).Namespace(ns)
	}, ls)
	if err != nil {
		return nil, errors.Wrap(err, "makeInformer")
	}
	watcher.Stop()

	options := []informers.SharedInformerOption{}
	if !ls.Empty() {
		options = append(options, informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.LabelSelector = ls.String()
		}))
	}
	if ns != "" {
		options = append(options, informers.WithNamespace(ns.String()))
	}

	factory := informers.NewSharedInformerFactoryWithOptions(kCli.clientSet, 5*time.Second, options...)
	resFactory, err := factory.ForResource(gvr)
	if err != nil {
		return nil, errors.Wrap(err, "makeInformer")
	}
	return resFactory.Informer(), nil
}

func (kCli K8sClient) WatchEvents(ctx context.Context) (<-chan *v1.Event, error) {
	gvr := v1.SchemeGroupVersion.WithResource("events")
	informer, err := kCli.makeInformer(ctx, gvr, labels.Everything())
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

	go informer.Run(ctx.Done())

	return ch, nil
}

func (kCli K8sClient) WatchPods(ctx context.Context, ls labels.Selector) (<-chan *v1.Pod, error) {
	gvr := v1.SchemeGroupVersion.WithResource("pods")
	informer, err := kCli.makeInformer(ctx, gvr, ls)
	if err != nil {
		return nil, errors.Wrap(err, "WatchPods")
	}

	ch := make(chan *v1.Pod)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj, ok := obj.(*v1.Pod)
			if ok {
				FixContainerStatusImages(mObj)
				ch <- mObj
			}
		},
		DeleteFunc: func(obj interface{}) {
			mObj, ok := obj.(*v1.Pod)
			if ok {
				FixContainerStatusImages(mObj)
				ch <- mObj
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			oldPod, ok := oldObj.(*v1.Pod)
			if !ok {
				return
			}

			newPod, ok := newObj.(*v1.Pod)
			if !ok {
				return
			}

			if oldPod != newPod {
				FixContainerStatusImages(newPod)
				ch <- newPod
			}
		},
	})

	go informer.Run(ctx.Done())

	return ch, nil
}

func (kCli K8sClient) WatchServices(ctx context.Context, lps []model.LabelPair) (<-chan *v1.Service, error) {
	ls := labels.SelectorFromSet(makeLabelSet(lps))
	gvr := v1.SchemeGroupVersion.WithResource("services")
	informer, err := kCli.makeInformer(ctx, gvr, ls)
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

	go informer.Run(ctx.Done())

	return ch, nil
}
