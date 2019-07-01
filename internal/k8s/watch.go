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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/windmilleng/tilt/internal/model"
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

func (kCli K8sClient) WatchEvents(ctx context.Context) (<-chan *v1.Event, error) {
	// HACK(dmiller): There's no way to get errors out of an informer. See https://github.com/kubernetes/client-go/issues/155
	// In the meantime, at least to get authorization and some other errors let's try to set up a watcher and then just
	// throw it away.
	watcher, _, err := kCli.makeWatcher(func(ns string) watcher {
		return kCli.core.Events(ns)
	}, labels.Everything())
	if err != nil {
		return nil, errors.Wrap(err, "error watching k8s events")
	}
	watcher.Stop()

	ch := make(chan *v1.Event)

	factory := informers.NewSharedInformerFactoryWithOptions(kCli.clientSet, 5*time.Second, informers.WithNamespace(kCli.configNamespace.String()))
	informer := factory.Core().V1().Events().Informer()

	stopper := make(chan struct{})

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

	go informer.Run(stopper)
	// TODO(dmiller): is this right?
	go func() {
		<-ctx.Done()
		close(stopper)
	}()

	return ch, nil
}

func (kCli K8sClient) WatchPods(ctx context.Context, ls labels.Selector) (<-chan *v1.Pod, error) {
	// HACK(dmiller): There's no way to get errors out of an informer. See https://github.com/kubernetes/client-go/issues/155
	// In the meantime, at least to get authorization and some other errors let's try to set up a watcher and then just
	// throw it away.
	watcher, ns, err := kCli.makeWatcher(func(ns string) watcher {
		return kCli.core.Pods(ns)
	}, ls)
	if err != nil {
		return nil, errors.Wrap(err, "pods.WatchFiles")
	}
	watcher.Stop()

	options := []informers.SharedInformerOption{}
	options = append(options, informers.WithTweakListOptions(func(o *metav1.ListOptions) {
		o.LabelSelector = ls.String()
	}))

	if ns != "" {
		options = append(options, informers.WithNamespace(ns.String()))
	}

	factory := informers.NewSharedInformerFactoryWithOptions(kCli.clientSet, 5*time.Second, options...)
	informer := factory.Core().V1().Pods().Informer()

	stopper := make(chan struct{})
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

	go informer.Run(stopper)
	// TODO(dmiller): is this right?
	go func() {
		<-ctx.Done()
		close(stopper)
	}()

	return ch, nil
}

func (kCli K8sClient) WatchServices(ctx context.Context, lps []model.LabelPair) (<-chan *v1.Service, error) {
	ch := make(chan *v1.Service)

	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}

	watcher, _, err := kCli.makeWatcher(func(ns string) watcher {
		return kCli.core.Services(ns)
	}, ls.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, "Services.WatchFiles")
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.ResultChan():
				if !ok {
					close(ch)
					return
				}

				if event.Object == nil {
					continue
				}

				service, ok := event.Object.(*v1.Service)
				if !ok {
					continue
				}

				ch <- service
			case <-ctx.Done():
				watcher.Stop()
				close(ch)
				return
			}
		}
	}()

	return ch, nil
}
