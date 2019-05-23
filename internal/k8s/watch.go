package k8s

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/windmilleng/tilt/internal/logger"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/model"

	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

type watcherFactory func(namespace string) watcher
type watcher interface {
	Watch(options metav1.ListOptions) (watch.Interface, error)
}

func (kCli K8sClient) makeWatcher(f watcherFactory, ls labels.Selector) (watch.Interface, Namespace, error) {
	// passing "" gets us all namespaces
	watcher, err := f("").Watch(metav1.ListOptions{LabelSelector: ls.String()})
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
		watcher, err := f(kCli.configNamespace.String()).Watch(metav1.ListOptions{LabelSelector: ls.String()})
		if err == nil {
			return watcher, kCli.configNamespace, nil
		}

		// ugh, it still failed. return the original error.
	}
	return nil, "", fmt.Errorf("%s, Reason: %s, Code: %d", status.Message, status.Reason, status.Code)
}

func (kCli K8sClient) WatchEvents(ctx context.Context, ls labels.Selector) (<-chan *v1.Event, error) {
	watcher, err := kCli.makeWatcher(func(ns string) watcher {
		return kCli.core.Events(ns)
	}, ls)
	if err != nil {
		return nil, errors.Wrap(err, "error watching k8s events")
	}
	watcher.Stop()

	ch := make(chan *v1.Event)

	factory := informers.NewSharedInformerFactoryWithOptions(kCli.clientSet, 5*time.Second)
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
				ch <- mObj
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

func (kCli K8sClient) WatchEverything(ctx context.Context, lps []model.LabelPair) (<-chan watch.Event, error) {
	ch := make(chan watch.Event)

	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}

	var watchers []watch.Interface

	_, resourceLists, err := kCli.clientSet.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting list of resource types")
	}

	var gvrs []schema.GroupVersionResource
	for _, rl := range resourceLists {
		// one might think we could use rl.GroupVersionKind().GroupVersion(), but that gave an empty `Group`
		// for most resources (one specific example in case we revisit this: statefulsets)
		rlGV, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing GroupVersion '%s'", rl.GroupVersion)
		}
		for _, r := range rl.APIResources {
			isWatchable := false
			for _, verb := range r.Verbs {
				if verb == "watch" {
					isWatchable = true
					break
				}
			}
			if !isWatchable {
				continue
			}
			group := r.Group
			if group == "" {
				group = rlGV.Group
			}
			version := r.Version
			if version == "" {
				version = rlGV.Version
			}
			gvrs = append(gvrs, schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: r.Name,
			})
		}
	}

	for _, gvr := range gvrs {
		watcher, _, err := kCli.makeWatcher(func(ns string) watcher {
			return kCli.dynamic.Resource(gvr)
		}, ls.AsSelector())

		if err != nil {
			return nil, errors.Wrapf(err, "error making watcher for resource '%s'", gvr.String())
		}

		watchers = append(watchers, watcher)
	}

	go func() {
		selectCases := []reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		}
		for _, w := range watchers {
			selectCases = append(selectCases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(w.ResultChan()),
			})
		}
		logger.Get(ctx).Infof("starting watch")
		for {
			chosen, value, ok := reflect.Select(selectCases)
			if chosen == 0 || !ok {
				logger.Get(ctx).Infof("stopping watch.")
				for _, w := range watchers {
					w.Stop()
				}
				close(ch)
				return
			}

			event := value.Interface().(watch.Event)
			if event.Object == nil {
				continue
			}

			ch <- event
		}
	}()

	return ch, nil
}
