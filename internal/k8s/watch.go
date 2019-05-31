package k8s

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"

	"github.com/pkg/errors"

	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/informers"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
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

func (kCli K8sClient) isWatchable(ctx context.Context, auth authorizationv1client.AuthorizationV1Interface, r metav1.APIResource, gv schema.GroupVersion) (bool, error) {
	// NOTE(maia): verb IS accounted for below, but this is an easy way to pare down the # of calls we make
	var hasWatchVerb bool
	for _, v := range r.Verbs {
		if v == "watch" {
			hasWatchVerb = true
		}
	}
	if !hasWatchVerb {
		return false, nil
	}

	// Based on `kubectl auth can-i`: https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go#L234
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: kCli.configNamespace.String(),
				Verb:      "watch",
				Group:     gv.Group,
				Resource:  r.Name,
			},
		},
	}

	resp, err := auth.SelfSubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}

	return resp.Status.Allowed, nil
}

// Get all GroupVersionResources in the cluster that support watching
func (kCli K8sClient) watchableGroupVersionResources(ctx context.Context) ([]schema.GroupVersionResource, error) {
	_, resourceLists, err := kCli.clientSet.Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting list of resource types")
	}

	var ret []schema.GroupVersionResource

	authCli := kCli.clientSet.AuthorizationV1()

	for _, rl := range resourceLists {
		// one might think it'd be cleaner to use rl.GroupVersionKind().GroupVersion()
		// empirically, but that returns an empty `Group` for most resources
		// (one specific example in case we revisit this: statefulsets)
		rlGV, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing GroupVersion '%s'", rl.GroupVersion)
		}
		for _, r := range rl.APIResources {
			watchable, err := kCli.isWatchable(ctx, authCli, r, rlGV)
			if err != nil {
				logger.Get(ctx).Infof("ERROR setting up watch for '%s.%s': %v", r.Name, rlGV.String(), err)
			}

			if !watchable {
				continue
			}

			// per comments on r.Group/r.Version: empty implies the value of the containing ResourceList
			group := r.Group
			if group == "" {
				group = rlGV.Group
			}
			version := r.Version
			if version == "" {
				version = rlGV.Version
			}
			ret = append(ret, schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: r.Name,
			})
		}
	}

	return ret, nil
}

func (kCli K8sClient) WatchEverything(ctx context.Context, lps []model.LabelPair) (<-chan watch.Event, error) {
	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}

	// there is no API to watch *everything*, but there is an API to watch everything of a given type
	// so we'll get the list of watchable types and make a watcher for each
	gvrs, err := kCli.watchableGroupVersionResources(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting list of watchable GroupVersionResources")
	}

	var watchers []watch.Interface
	for _, gvr := range gvrs {
		watcher, _, err := kCli.makeWatcher(func(ns string) watcher {
			return kCli.dynamic.Resource(gvr)
		}, ls.AsSelector())

		if err != nil {
			return nil, errors.Wrapf(err, "error making watcher for resource '%s'", gvr.String())
		}

		watchers = append(watchers, watcher)
	}

	ch := make(chan watch.Event)

	go watchEverythingLoop(ctx, ch, watchers, gvrs)

	return ch, nil
}

func watchEverythingLoop(ctx context.Context, ch chan<- watch.Event, watchers []watch.Interface, gvrs []schema.GroupVersionResource) {
	var selectCases []reflect.SelectCase
	for _, w := range watchers {
		selectCases = append(selectCases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(w.ResultChan()),
		})
	}

	selectCases = append(selectCases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())})

	for {
		chosen, value, ok := reflect.Select(selectCases)

		// the last selectCase is ctx.Done
		if chosen == len(selectCases)-1 {
			cleanUp(ch, watchers)
			return
		}

		if !ok {
			// XXX DEBUG
			// for some reason, we're getting ok = false for fission resources (e.g. fission.io/v1, Resource=environments)
			// This happens after running tilt for 10-20 seconds
			// My current hypotheses:
			// 1. A misunderstanding on my part of what the third return value from `reflect.Select` indicates
			// 2. A bug in the watch implementation for fission CRDs
			// 3. A misunderstanding of how the watch API is to be used (maybe periodic closes are to be expected,
			//    and we should just restart the watch? AFAIK, we haven't seen this with our existing watches,
			//    but, then again our logging allowed this to exist unnoticed for quite some time:
			//    https://github.com/windmilleng/tilt/pull/1647. I feel like we would have noticed this with pods,
			//    though.
			// 4. Maybe setting up 100+ watches taxes the system and causes it to drop connections
			logger.Get(ctx).Infof("DEBUG: ok was false for %v", gvrs[chosen])
			cleanUp(ch, watchers)
			return
		}

		event := value.Interface().(watch.Event)
		if event.Object == nil {
			continue
		}

		ch <- event
	}
}

func cleanUp(ch chan<- watch.Event, watchers []watch.Interface) {
	for _, w := range watchers {
		w.Stop()
	}
	close(ch)
	return
}
