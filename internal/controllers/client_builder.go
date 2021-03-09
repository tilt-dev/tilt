package controllers

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

type ClientBuilder struct {
	cluster.ClientBuilder
	deferred *DeferredClient
}

func NewClientBuilder(deferred *DeferredClient) cluster.ClientBuilder {
	return &ClientBuilder{
		ClientBuilder: cluster.NewClientBuilder(),
		deferred:      deferred,
	}
}

func (b *ClientBuilder) Build(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	c, err := b.ClientBuilder.Build(cache, config, options)
	if err != nil {
		return nil, err
	}
	b.deferred.initialize(c)
	return c, nil
}
