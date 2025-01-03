/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	utilversion "k8s.io/component-base/version"
)

func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	// Loosely adapted from
	// https://github.com/kubernetes/apiextensions-apiserver/blob/a1e0ff9923b68731a7735f1c4168b6b3d0bd1027/pkg/apiserver/apiserver.go#L57
	//
	// Whitelist the unversioned API types
	// for any apiserver.

	unversioned := schema.GroupVersion{
		Group:   "",
		Version: "v1",
	}
	metav1.AddToGroupVersion(scheme, unversioned)

	scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.WatchEvent{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
	return scheme
}

// ExtraConfig holds custom apiserver config
type ExtraConfig struct {
	Scheme         *runtime.Scheme
	Codecs         serializer.CodecFactory
	APIs           map[schema.GroupVersionResource]StorageProvider
	ServingInfo    *genericapiserver.SecureServingInfo
	Version        *version.Info
	ParameterCodec runtime.ParameterCodec
}

// Config defines the config for the apiserver
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// TiltServer contains state for a Kubernetes cluster master/api server.
type TiltServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside of this package.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	v := utilversion.DefaultKubeEffectiveVersion()
	cfg.GenericConfig.EffectiveVersion = v

	c := completedConfig{}
	c.GenericConfig = cfg.GenericConfig.Complete()
	c.ExtraConfig = &cfg.ExtraConfig
	return CompletedConfig{&c}
}

// New returns a new instance of TiltServer from the given config.
func (c completedConfig) New() (*TiltServer, error) {
	genericServer, err := c.GenericConfig.New("tilt-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	s := &TiltServer{
		GenericAPIServer: genericServer,
	}

	// Add new APIs through inserting into APIs
	apiGroups, err := buildAPIGroupInfos(c.ExtraConfig.Scheme, c.ExtraConfig.Codecs, c.ExtraConfig.APIs, c.GenericConfig.RESTOptionsGetter, c.ExtraConfig.ParameterCodec)
	if err != nil {
		return nil, err
	}
	for _, apiGroup := range apiGroups {
		if err := s.GenericAPIServer.InstallAPIGroup(apiGroup); err != nil {
			return nil, err
		}
	}

	return s, nil
}
