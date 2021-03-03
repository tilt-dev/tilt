/*
Copyright 2020 The Tilt Dev Authors

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
)

// GroupName is the group name used in this package
const GroupName = "core.tilt.dev"

const Version = "v1alpha1"

// The label on any object that identifies which manifest
// its logs should appear under.
const LabelManifest = "resource"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

func AllResourceObjects() []resource.Object {
	return []resource.Object{
		&Manifest{},
		&FileWatch{},
		&Cmd{},

		// Hey! You! If you're adding a new top-level type, add the type object here.
	}
}
func AllResourceLists() []runtime.Object {
	return []runtime.Object{
		&ManifestList{},
		&FileWatchList{},
		&CmdList{},

		// Hey! You! If you're adding a new top-level type, add the List type here.
	}
}

var AddToScheme = func(scheme *runtime.Scheme) error {
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{
		Group:   GroupName,
		Version: Version,
	})

	objs := []runtime.Object{}
	for _, obj := range AllResourceObjects() {
		objs = append(objs, obj)
	}
	objs = append(objs, AllResourceLists()...)

	scheme.AddKnownTypes(schema.GroupVersion{
		Group:   GroupName,
		Version: Version,
	}, objs...)
	return nil
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
