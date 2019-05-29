package main

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/davecgh/go-spew/spew"
	"k8s.io/client-go/kubernetes"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func newClient() (clientv1.CoreV1Interface, *rest.Config, *kubernetes.Clientset, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{}
	clientLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		overrides)

	config, err := clientLoader.ClientConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	return clientset.CoreV1(), config, clientset, nil
}

func run() error {
	_, restConfig, _, err := newClient()
	if err != nil {
		return err
	}

	//factory := informers.NewSharedInformerFactoryWithOptions(clientSet, 5*time.Second)
	//informer := factory.Core().V1().Events().Informer()
	//
	//stopper := make(chan struct{})
	//
	//informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: func(obj interface{}) {
	//		fmt.Printf("new event: '%s'\n", spew.Sdump(obj))
	//	},
	//	UpdateFunc: func(oldObj interface{}, newObj interface{}) {
	//		fmt.Printf("new event: '%s'\n", spew.Sdump(newObj))
	//	},
	//})
	//
	//go informer.Run(stopper)

	di := dynamic.NewForConfigOrDie(restConfig)

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	watch, err := di.Resource(gvr).Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}

	if watch == nil {
		fmt.Printf("watch is nil!\n")
		return nil
	}

	go func() {
		for {
			ev, ok := <-watch.ResultChan()
			if !ok {
				fmt.Printf("ResultChan closed\n")
				break
			}
			fmt.Printf("%s: %s\n", ev.Type, spew.Sdump(ev.Object))
		}
	}()

	fmt.Println("listening")

	var input string
	_, _ = fmt.Scanln(&input)

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
