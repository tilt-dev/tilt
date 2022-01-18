package k8s

import (
	"io"
	"io/ioutil"
	"strings"

	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// We've adapted Helm's kubernetes client for our needs
type ResourceClient interface {
	Apply(target kube.ResourceList) (*kube.Result, error)
	CreateOrReplace(target kube.ResourceList) (*kube.Result, error)
	Delete(existing kube.ResourceList) (*kube.Result, []error)
	Create(l kube.ResourceList) (*kube.Result, error)
	Build(r io.Reader, validate bool) (kube.ResourceList, error)
}

type resourceClient struct {
	*kube.Client
	factory cmdutil.Factory
}

// Helm's update function doesn't really work for us,
// so we use the kubectl apply code directly.
func (c *resourceClient) Apply(target kube.ResourceList) (*kube.Result, error) {
	f := c.factory
	iostreams := genericclioptions.IOStreams{
		In:     strings.NewReader(""),
		Out:    ioutil.Discard,
		ErrOut: ioutil.Discard,
	}
	flags := apply.NewApplyFlags(f, iostreams)

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return nil, err
	}

	recorder, err := genericclioptions.NewRecordFlags().ToRecorder()
	if err != nil {
		return nil, err
	}
	deleteOptions, err := delete.NewDeleteFlags("").ToOptions(dynamicClient, iostreams)
	if err != nil {
		return nil, err
	}
	toPrinter := func(s string) (printers.ResourcePrinter, error) {
		return genericclioptions.NewPrintFlags("created").ToPrinter()
	}
	openAPISchema, _ := f.OpenAPISchema()
	builder := f.NewBuilder()
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	namespace, enforceNamespace, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}

	o := &apply.ApplyOptions{
		PrintFlags: flags.PrintFlags,

		DeleteOptions:  deleteOptions,
		ToPrinter:      toPrinter,
		Selector:       flags.Selector,
		Prune:          flags.Prune,
		PruneResources: flags.PruneResources,
		All:            flags.All,
		Overwrite:      flags.Overwrite,
		OpenAPIPatch:   flags.OpenAPIPatch,
		PruneWhitelist: flags.PruneWhitelist,

		Recorder:         recorder,
		Namespace:        namespace,
		EnforceNamespace: enforceNamespace,
		Builder:          builder,
		Mapper:           mapper,
		DynamicClient:    dynamicClient,
		OpenAPISchema:    openAPISchema,

		IOStreams: flags.IOStreams,

		VisitedUids:       sets.NewString(),
		VisitedNamespaces: sets.NewString(),
	}

	o.SetObjects(target)
	err = o.Run()
	if err != nil {
		return nil, err
	}
	return &kube.Result{Updated: target}, nil
}

// A simplified implementation that creates or replaces the whole object.
func (c *resourceClient) CreateOrReplace(target kube.ResourceList) (*kube.Result, error) {
	for _, info := range target {
		obj, err := resource.
			NewHelper(info.Client, info.Mapping).
			Create(info.Namespace, true, info.Object)

		if err != nil && strings.Contains(err.Error(), "already exists") {
			obj, err = resource.
				NewHelper(info.Client, info.Mapping).
				Replace(info.Namespace, info.Name, true, info.Object)
		}

		if err != nil {
			return nil, cmdutil.AddSourceToErr("create/replace", info.Source, err)
		}

		err = info.Refresh(obj, true)
		if err != nil {
			return nil, cmdutil.AddSourceToErr("create/replace", info.Source, err)
		}
	}

	return &kube.Result{Updated: target}, nil
}

var helmNopLogger = func(_ string, _ ...interface{}) {}

func newResourceClient(c *K8sClient) ResourceClient {
	f := cmdutil.NewFactory(c)

	// Don't use kube.New() here, because it modifies globals in
	// a way that breaks tests.
	return &resourceClient{
		Client: &kube.Client{
			Factory: f,
			Log:     helmNopLogger,
		},
		factory: f,
	}
}
