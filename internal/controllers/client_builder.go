package controllers

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

type ClientBuilder struct {
	delegate cluster.ClientBuilder
	deferred *DeferredClient
}

func NewClientBuilder(deferred *DeferredClient) cluster.ClientBuilder {
	return ClientBuilder{
		delegate: cluster.NewClientBuilder(),
		deferred: deferred,
	}
}

func (b ClientBuilder) WithUncached(objs ...client.Object) cluster.ClientBuilder {
	b.delegate = b.delegate.WithUncached(objs...)
	return b
}

func (b ClientBuilder) Build(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	c, err := b.delegate.Build(cache, config, options)
	if err != nil {
		return nil, err
	}
	b.deferred.initialize(c)
	return c, nil
}
