package cli

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/dynamic"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
)

// Helper for human-friendly CLI for creating objects.
//
// See other create commands for usage examples.
type createHelper struct {
	streams       genericclioptions.IOStreams
	printFlags    *genericclioptions.PrintFlags
	dynamicClient dynamic.Interface
	printer       printers.ResourcePrinter
}

func newCreateHelper() *createHelper {
	streams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr, In: os.Stdin}
	return &createHelper{
		streams:    streams,
		printFlags: genericclioptions.NewPrintFlags("created"),
	}
}

func (h *createHelper) addFlags(cmd *cobra.Command) {
	h.printFlags.AddFlags(cmd)
	addConnectServerFlags(cmd)
}

func (h *createHelper) interpretFlags(ctx context.Context) error {
	printer, err := h.printFlags.ToPrinter()
	if err != nil {
		return err
	}
	h.printer = printer

	dynamicClient, err := h.createDynamicClient(ctx)
	if err != nil {
		return err
	}
	h.dynamicClient = dynamicClient
	return nil
}

func (h *createHelper) create(ctx context.Context, resourceObj resource.Object) error {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resourceObj)
	if err != nil {
		return err
	}

	result, err := h.dynamicClient.Resource(resourceObj.GetGroupVersionResource()).
		Create(ctx, &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return h.printer.PrintObj(result, h.streams.Out)
}

// Loads a dynamically typed tilt client.
func (h *createHelper) createDynamicClient(ctx context.Context) (dynamic.Interface, error) {
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return nil, err
	}

	config, err := getter.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}
