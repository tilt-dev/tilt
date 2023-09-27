/*
Copyright 2017 The Kubernetes Authors.

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

package start

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	pkgserver "k8s.io/apiserver/pkg/server"
	openapicommon "k8s.io/kube-openapi/pkg/common"
)

type RecommendedConfigFn func(*pkgserver.RecommendedConfig) *pkgserver.RecommendedConfig

func (o *TiltServerOptions) ApplyRecommendedConfigFns(in *pkgserver.RecommendedConfig) *pkgserver.RecommendedConfig {
	for i := range o.recommendedConfigFns {
		in = o.recommendedConfigFns[i](in)
	}
	return in
}

func SetOpenAPIDefinitionFn(scheme *runtime.Scheme, name, version string, defs openapicommon.GetOpenAPIDefinitions) RecommendedConfigFn {
	return RecommendedConfigFn(func(config *pkgserver.RecommendedConfig) *pkgserver.RecommendedConfig {
		config.OpenAPIV3Config = pkgserver.DefaultOpenAPIV3Config(defs, openapi.NewDefinitionNamer(scheme))
		config.OpenAPIV3Config.Info.Title = name
		config.OpenAPIV3Config.Info.Version = version

		config.OpenAPIConfig = pkgserver.DefaultOpenAPIConfig(defs, openapi.NewDefinitionNamer(scheme))
		config.OpenAPIConfig.Info.Title = name
		config.OpenAPIConfig.Info.Version = version
		return config
	})
}
