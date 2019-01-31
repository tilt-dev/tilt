package k8s

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (kCli K8sClient) WatchPods(ctx context.Context, ls labels.Set) (<-chan *v1.Pod, error) {
	ch := make(chan *v1.Pod)

	// passing "" gets us all namespaces
	watcher, err := kCli.core.Pods("").Watch(metav1.ListOptions{LabelSelector: ls.String()})
	if err != nil {
		return nil, err
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

	// passing "" gets us all namespaces
	watcher, err := kCli.core.Services("").Watch(metav1.ListOptions{LabelSelector: ls.String()})
	if err != nil {
		return nil, err
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
