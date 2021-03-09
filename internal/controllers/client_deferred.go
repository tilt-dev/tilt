package controllers

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrClientNotInitialized = errors.New("controller client not initialized")

type DeferredClient struct {
	impl atomic.Value
}

var _ ctrlclient.Client = &DeferredClient{}

func ProvideDeferredClient() *DeferredClient {
	return &DeferredClient{}
}

func (d *DeferredClient) initialize(client ctrlclient.Client) {
	d.impl.Store(client)
}

func (d *DeferredClient) client() ctrlclient.Client {
	v := d.impl.Load()
	if v == nil {
		return nil
	}
	cli, ok := v.(ctrlclient.Client)
	if !ok {
		panic(fmt.Errorf("deferred client initialized with bad type: %T", v))
	}
	return cli
}

func (d *DeferredClient) Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object) error {
	cli := d.client()
	if cli == nil {
		return ErrClientNotInitialized
	}
	return cli.Get(ctx, key, obj)
}

func (d *DeferredClient) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	cli := d.client()
	if cli == nil {
		return ErrClientNotInitialized
	}
	return cli.List(ctx, list, opts...)
}

func (d *DeferredClient) Create(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error {
	cli := d.client()
	if cli == nil {
		return ErrClientNotInitialized
	}
	return cli.Create(ctx, obj, opts...)
}

func (d *DeferredClient) Delete(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error {
	cli := d.client()
	if cli == nil {
		return ErrClientNotInitialized
	}
	return cli.Delete(ctx, obj, opts...)
}

func (d *DeferredClient) Update(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.UpdateOption) error {
	cli := d.client()
	if cli == nil {
		return ErrClientNotInitialized
	}
	return cli.Update(ctx, obj, opts...)
}

func (d *DeferredClient) Patch(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
	cli := d.client()
	if cli == nil {
		return ErrClientNotInitialized
	}
	return cli.Patch(ctx, obj, patch, opts...)
}

func (d *DeferredClient) DeleteAllOf(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteAllOfOption) error {
	cli := d.client()
	if cli == nil {
		return ErrClientNotInitialized
	}
	return cli.DeleteAllOf(ctx, obj, opts...)
}

func (d *DeferredClient) Status() ctrlclient.StatusWriter {
	cli := d.client()
	if cli == nil {
		return nil
	}
	return cli.Status()
}

func (d *DeferredClient) Scheme() *runtime.Scheme {
	cli := d.client()
	if cli == nil {
		return nil
	}
	return cli.Scheme()
}

func (d *DeferredClient) RESTMapper() meta.RESTMapper {
	cli := d.client()
	if cli == nil {
		return nil
	}
	return cli.RESTMapper()
}
