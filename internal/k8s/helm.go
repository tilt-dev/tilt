package k8s

import (
	"io"
	"io/ioutil"
	"strings"

	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// We've adapted Helm's kubernetes client for our needs
type HelmKubeClient interface {
	Apply(target kube.ResourceList) (*kube.Result, error)
	Delete(existing kube.ResourceList) (*kube.Result, []error)
	Create(l kube.ResourceList) (*kube.Result, error)
	Build(r io.Reader, validate bool) (kube.ResourceList, error)
}

type helmKubeClient struct {
	*kube.Client
	factory cmdutil.Factory
}

// Helm's update function doesn't really work for us,
// so we use the kubectl apply code directly.
func (c *helmKubeClient) Apply(target kube.ResourceList) (*kube.Result, error) {
	f := c.factory
	o := apply.NewApplyOptions(genericclioptions.IOStreams{
		In:     strings.NewReader(""),
		Out:    ioutil.Discard,
		ErrOut: ioutil.Discard,
	})

	var err error
	o.DynamicClient, err = f.DynamicClient()
	if err != nil {
		return nil, err
	}

	deleteOptions, _ := delete.NewDeleteFlags("").ToOptions(o.DynamicClient, o.IOStreams)
	o.DeleteOptions = deleteOptions
	o.ToPrinter = func(s string) (printers.ResourcePrinter, error) {
		return genericclioptions.NewPrintFlags("created").ToPrinter()
	}
	o.Builder = f.NewBuilder()
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}

	o.SetObjects(target)
	err = o.Run()
	if err != nil {
		return nil, err
	}
	return &kube.Result{Updated: target}, nil
}

func newHelmKubeClient(c *K8sClient) HelmKubeClient {
	return &helmKubeClient{
		Client:  kube.New(c),
		factory: cmdutil.NewFactory(c),
	}
}
