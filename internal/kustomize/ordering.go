package kustomize

/**
Code for ordering Kubernetes entities by kind.

Adapted from
https://github.com/kubernetes-sigs/kustomize/blob/180429774a5fefab0d6af9ada7f866c177b5d7b4/pkg/gvk/gvk.go#L82

Copyright 2019 The Kubernetes Authors.

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

// An attempt to order things to help k8s, e.g.
// a Service should come before things that refer to it.
// Namespace should be first.
// In some cases order just specified to provide determinism.
// Adapted from Kustomize: https://github.com/kubernetes-sigs/kustomize/blob/180429774a5fefab0d6af9ada7f866c177b5d7b4/pkg/gvk/gvk.go#L82
var OrderFirst = []string{
	"Namespace",
	"StorageClass",
	"CustomResourceDefinition",
	"MutatingWebhookConfiguration",
	"ServiceAccount",
	"PodSecurityPolicy",
	"Role",
	"ClusterRole",
	"RoleBinding",
	"ClusterRoleBinding",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"ConfigMap",
	"Secret",
	"Service",
	"LimitRange",
	"Deployment",
	"StatefulSet",
	"CronJob",
	"PodDisruptionBudget",
}

var TypeOrders = func() map[string]int {
	m := map[string]int{}
	for i, n := range OrderFirst {
		m[n] = -len(OrderFirst) + i
	}
	return m
}()
