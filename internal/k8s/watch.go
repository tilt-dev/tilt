package k8s

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/model"

	"k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
)

type watcherFactory func(namespace string) watcher
type watcher interface {
	Watch(options metav1.ListOptions) (watch.Interface, error)
}

func (kCli K8sClient) makeWatcher(f watcherFactory, ls labels.Selector) (watch.Interface, error) {
	// passing "" gets us all namespaces
	watcher, err := f("").Watch(metav1.ListOptions{LabelSelector: ls.String()})
	if err == nil {
		return watcher, nil
	}

	// If the request failed, we might be able to recover.
	statusErr, isStatusErr := err.(*apiErrors.StatusError)
	if !isStatusErr {
		return nil, err
	}

	status := statusErr.ErrStatus
	if status.Code == http.StatusForbidden {
		// If this is a forbidden error, maybe the user just isn't allowed to watch this namespace.
		// Let's narrow our request to just the config namespace, and see if that helps.
		watcher, err := f(kCli.configNamespace.String()).Watch(metav1.ListOptions{LabelSelector: ls.String()})
		if err == nil {
			return watcher, nil
		}

		// ugh, it still failed. return the original error.
	}
	return nil, fmt.Errorf("%s, Reason: %s, Code: %d", status.Message, status.Reason, status.Code)
}

func (kCli K8sClient) WatchPods(ctx context.Context, ls labels.Selector) (<-chan *v1.Pod, error) {
	ch := make(chan *v1.Pod)
	watcher, err := kCli.makeWatcher(func(ns string) watcher {
		return kCli.core.Pods(ns)
	}, ls)
	if err != nil {
		return nil, errors.Wrap(err, "Pods.Watch")
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

				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					continue
				}

				ch <- pod
			case <-ctx.Done():
				watcher.Stop()
				close(ch)
				return
			}
		}
	}()

	return ch, nil
}

func (kCli K8sClient) WatchServices(ctx context.Context, lps []model.LabelPair) (<-chan *v1.Service, error) {
	ch := make(chan *v1.Service)

	ls := labels.Set{}
	for _, lp := range lps {
		ls[lp.Key] = lp.Value
	}

	watcher, err := kCli.makeWatcher(func(ns string) watcher {
		return kCli.core.Services(ns)
	}, ls.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, "Services.Watch")
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
