/*
Copyright 2018 The Kubernetes Authors.

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

package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/internal/objectutil"
)

type versionedTracker struct {
	testing.ObjectTracker
	scheme *runtime.Scheme
}

type fakeClient struct {
	tracker versionedTracker
	scheme  *runtime.Scheme
}

var _ client.Client = &fakeClient{}

const (
	maxNameLength          = 63
	randomLength           = 5
	maxGeneratedNameLength = maxNameLength - randomLength
)

// NewFakeClient creates a new fake client for testing.
// You can choose to initialize it with a slice of runtime.Object.
//
// Deprecated: Please use NewClientBuilder instead.
func NewFakeClient(initObjs ...runtime.Object) client.Client {
	return NewClientBuilder().WithRuntimeObjects(initObjs...).Build()
}

// NewFakeClientWithScheme creates a new fake client with the given scheme
// for testing.
// You can choose to initialize it with a slice of runtime.Object.
//
// Deprecated: Please use NewClientBuilder instead.
func NewFakeClientWithScheme(clientScheme *runtime.Scheme, initObjs ...runtime.Object) client.Client {
	return NewClientBuilder().WithScheme(clientScheme).WithRuntimeObjects(initObjs...).Build()
}

// NewClientBuilder returns a new builder to create a fake client.
func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{}
}

// ClientBuilder builds a fake client.
type ClientBuilder struct {
	scheme             *runtime.Scheme
	initObject         []client.Object
	initLists          []client.ObjectList
	initRuntimeObjects []runtime.Object
}

// WithScheme sets this builder's internal scheme.
// If not set, defaults to client-go's global scheme.Scheme.
func (f *ClientBuilder) WithScheme(scheme *runtime.Scheme) *ClientBuilder {
	f.scheme = scheme
	return f
}

// WithObjects can be optionally used to initialize this fake client with client.Object(s).
func (f *ClientBuilder) WithObjects(initObjs ...client.Object) *ClientBuilder {
	f.initObject = append(f.initObject, initObjs...)
	return f
}

// WithLists can be optionally used to initialize this fake client with client.ObjectList(s).
func (f *ClientBuilder) WithLists(initLists ...client.ObjectList) *ClientBuilder {
	f.initLists = append(f.initLists, initLists...)
	return f
}

// WithRuntimeObjects can be optionally used to initialize this fake client with runtime.Object(s).
func (f *ClientBuilder) WithRuntimeObjects(initRuntimeObjs ...runtime.Object) *ClientBuilder {
	f.initRuntimeObjects = append(f.initRuntimeObjects, initRuntimeObjs...)
	return f
}

// Build builds and returns a new fake client.
func (f *ClientBuilder) Build() client.Client {
	if f.scheme == nil {
		f.scheme = scheme.Scheme
	}

	tracker := versionedTracker{ObjectTracker: testing.NewObjectTracker(f.scheme, scheme.Codecs.UniversalDecoder()), scheme: f.scheme}
	for _, obj := range f.initObject {
		if err := tracker.Add(obj); err != nil {
			panic(fmt.Errorf("failed to add object %v to fake client: %w", obj, err))
		}
	}
	for _, obj := range f.initLists {
		if err := tracker.Add(obj); err != nil {
			panic(fmt.Errorf("failed to add list %v to fake client: %w", obj, err))
		}
	}
	for _, obj := range f.initRuntimeObjects {
		if err := tracker.Add(obj); err != nil {
			panic(fmt.Errorf("failed to add runtime object %v to fake client: %w", obj, err))
		}
	}
	return &fakeClient{
		tracker: tracker,
		scheme:  f.scheme,
	}
}

const trackerAddResourceVersion = "999"

func (t versionedTracker) Add(obj runtime.Object) error {
	var objects []runtime.Object
	if meta.IsListType(obj) {
		var err error
		objects, err = meta.ExtractList(obj)
		if err != nil {
			return err
		}
	} else {
		objects = []runtime.Object{obj}
	}
	for _, obj := range objects {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return fmt.Errorf("failed to get accessor for object: %w", err)
		}
		if accessor.GetResourceVersion() == "" {
			// We use a "magic" value of 999 here because this field
			// is parsed as uint and and 0 is already used in Update.
			// As we can't go lower, go very high instead so this can
			// be recognized
			accessor.SetResourceVersion(trackerAddResourceVersion)
		}
		if err := t.ObjectTracker.Add(obj); err != nil {
			return err
		}
	}

	return nil
}

func (t versionedTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("failed to get accessor for object: %v", err)
	}
	if accessor.GetName() == "" {
		return apierrors.NewInvalid(
			obj.GetObjectKind().GroupVersionKind().GroupKind(),
			accessor.GetName(),
			field.ErrorList{field.Required(field.NewPath("metadata.name"), "name is required")})
	}
	if accessor.GetResourceVersion() != "" {
		return apierrors.NewBadRequest("resourceVersion can not be set for Create requests")
	}
	accessor.SetResourceVersion("1")
	if err := t.ObjectTracker.Create(gvr, obj, ns); err != nil {
		accessor.SetResourceVersion("")
		return err
	}
	return nil
}

func (t versionedTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("failed to get accessor for object: %v", err)
	}

	if accessor.GetName() == "" {
		return apierrors.NewInvalid(
			obj.GetObjectKind().GroupVersionKind().GroupKind(),
			accessor.GetName(),
			field.ErrorList{field.Required(field.NewPath("metadata.name"), "name is required")})
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		gvk, err = apiutil.GVKForObject(obj, t.scheme)
		if err != nil {
			return err
		}
	}

	oldObject, err := t.ObjectTracker.Get(gvr, ns, accessor.GetName())
	if err != nil {
		// If the resource is not found and the resource allows create on update, issue a
		// create instead.
		if apierrors.IsNotFound(err) && allowsCreateOnUpdate(gvk) {
			return t.Create(gvr, obj, ns)
		}
		return err
	}

	oldAccessor, err := meta.Accessor(oldObject)
	if err != nil {
		return err
	}

	// If the new object does not have the resource version set and it allows unconditional update,
	// default it to the resource version of the existing resource
	if accessor.GetResourceVersion() == "" && allowsUnconditionalUpdate(gvk) {
		accessor.SetResourceVersion(oldAccessor.GetResourceVersion())
	}
	if accessor.GetResourceVersion() != oldAccessor.GetResourceVersion() {
		return apierrors.NewConflict(gvr.GroupResource(), accessor.GetName(), errors.New("object was modified"))
	}
	if oldAccessor.GetResourceVersion() == "" {
		oldAccessor.SetResourceVersion("0")
	}
	intResourceVersion, err := strconv.ParseUint(oldAccessor.GetResourceVersion(), 10, 64)
	if err != nil {
		return fmt.Errorf("can not convert resourceVersion %q to int: %v", oldAccessor.GetResourceVersion(), err)
	}
	intResourceVersion++
	accessor.SetResourceVersion(strconv.FormatUint(intResourceVersion, 10))
	return t.ObjectTracker.Update(gvr, obj, ns)
}

func (c *fakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	o, err := c.tracker.Get(gvr, key.Namespace, key.Name)
	if err != nil {
		return err
	}

	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return err
	}
	ta, err := meta.TypeAccessor(o)
	if err != nil {
		return err
	}
	ta.SetKind(gvk.Kind)
	ta.SetAPIVersion(gvk.GroupVersion().String())

	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, obj)
	return err
}

func (c *fakeClient) List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return err
	}

	OriginalKind := gvk.Kind

	if !strings.HasSuffix(gvk.Kind, "List") {
		return fmt.Errorf("non-list type %T (kind %q) passed as output", obj, gvk)
	}
	// we need the non-list GVK, so chop off the "List" from the end of the kind
	gvk.Kind = gvk.Kind[:len(gvk.Kind)-4]

	listOpts := client.ListOptions{}
	listOpts.ApplyOptions(opts)

	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	o, err := c.tracker.List(gvr, gvk, listOpts.Namespace)
	if err != nil {
		return err
	}

	ta, err := meta.TypeAccessor(o)
	if err != nil {
		return err
	}
	ta.SetKind(OriginalKind)
	ta.SetAPIVersion(gvk.GroupVersion().String())

	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, obj)
	if err != nil {
		return err
	}

	if listOpts.LabelSelector != nil {
		objs, err := meta.ExtractList(obj)
		if err != nil {
			return err
		}
		filteredObjs, err := objectutil.FilterWithLabels(objs, listOpts.LabelSelector)
		if err != nil {
			return err
		}
		err = meta.SetList(obj, filteredObjs)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *fakeClient) Scheme() *runtime.Scheme {
	return c.scheme
}

func (c *fakeClient) RESTMapper() meta.RESTMapper {
	// TODO: Implement a fake RESTMapper.
	return nil
}

func (c *fakeClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	createOptions := &client.CreateOptions{}
	createOptions.ApplyOptions(opts)

	for _, dryRunOpt := range createOptions.DryRun {
		if dryRunOpt == metav1.DryRunAll {
			return nil
		}
	}

	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	if accessor.GetName() == "" && accessor.GetGenerateName() != "" {
		base := accessor.GetGenerateName()
		if len(base) > maxGeneratedNameLength {
			base = base[:maxGeneratedNameLength]
		}
		accessor.SetName(fmt.Sprintf("%s%s", base, utilrand.String(randomLength)))
	}

	return c.tracker.Create(gvr, obj, accessor.GetNamespace())
}

func (c *fakeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	delOptions := client.DeleteOptions{}
	delOptions.ApplyOptions(opts)

	//TODO: implement propagation
	return c.tracker.Delete(gvr, accessor.GetNamespace(), accessor.GetName())
}

func (c *fakeClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return err
	}

	dcOptions := client.DeleteAllOfOptions{}
	dcOptions.ApplyOptions(opts)

	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	o, err := c.tracker.List(gvr, gvk, dcOptions.Namespace)
	if err != nil {
		return err
	}

	objs, err := meta.ExtractList(o)
	if err != nil {
		return err
	}
	filteredObjs, err := objectutil.FilterWithLabels(objs, dcOptions.LabelSelector)
	if err != nil {
		return err
	}
	for _, o := range filteredObjs {
		accessor, err := meta.Accessor(o)
		if err != nil {
			return err
		}
		err = c.tracker.Delete(gvr, accessor.GetNamespace(), accessor.GetName())
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *fakeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	updateOptions := &client.UpdateOptions{}
	updateOptions.ApplyOptions(opts)

	for _, dryRunOpt := range updateOptions.DryRun {
		if dryRunOpt == metav1.DryRunAll {
			return nil
		}
	}

	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	return c.tracker.Update(gvr, obj, accessor.GetNamespace())
}

func (c *fakeClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	patchOptions := &client.PatchOptions{}
	patchOptions.ApplyOptions(opts)

	for _, dryRunOpt := range patchOptions.DryRun {
		if dryRunOpt == metav1.DryRunAll {
			return nil
		}
	}

	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	data, err := patch.Data(obj)
	if err != nil {
		return err
	}

	reaction := testing.ObjectReaction(c.tracker)
	handled, o, err := reaction(testing.NewPatchAction(gvr, accessor.GetNamespace(), accessor.GetName(), patch.Type(), data))
	if err != nil {
		return err
	}
	if !handled {
		panic("tracker could not handle patch method")
	}

	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return err
	}
	ta, err := meta.TypeAccessor(o)
	if err != nil {
		return err
	}
	ta.SetKind(gvk.Kind)
	ta.SetAPIVersion(gvk.GroupVersion().String())

	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, obj)
	return err
}

func (c *fakeClient) Status() client.StatusWriter {
	return &fakeStatusWriter{client: c}
}

func getGVRFromObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionResource, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return gvr, nil
}

type fakeStatusWriter struct {
	client *fakeClient
}

func (sw *fakeStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	// TODO(droot): This results in full update of the obj (spec + status). Need
	// a way to update status field only.
	return sw.client.Update(ctx, obj, opts...)
}

func (sw *fakeStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	// TODO(droot): This results in full update of the obj (spec + status). Need
	// a way to update status field only.
	return sw.client.Patch(ctx, obj, patch, opts...)
}

func allowsUnconditionalUpdate(gvk schema.GroupVersionKind) bool {
	switch gvk.Group {
	case "apps":
		switch gvk.Kind {
		case "ControllerRevision", "DaemonSet", "Deployment", "ReplicaSet", "StatefulSet":
			return true
		}
	case "autoscaling":
		switch gvk.Kind {
		case "HorizontalPodAutoscaler":
			return true
		}
	case "batch":
		switch gvk.Kind {
		case "CronJob", "Job":
			return true
		}
	case "certificates":
		switch gvk.Kind {
		case "Certificates":
			return true
		}
	case "flowcontrol":
		switch gvk.Kind {
		case "FlowSchema", "PriorityLevelConfiguration":
			return true
		}
	case "networking":
		switch gvk.Kind {
		case "Ingress", "IngressClass", "NetworkPolicy":
			return true
		}
	case "policy":
		switch gvk.Kind {
		case "PodSecurityPolicy":
			return true
		}
	case "rbac":
		switch gvk.Kind {
		case "ClusterRole", "ClusterRoleBinding", "Role", "RoleBinding":
			return true
		}
	case "scheduling":
		switch gvk.Kind {
		case "PriorityClass":
			return true
		}
	case "settings":
		switch gvk.Kind {
		case "PodPreset":
			return true
		}
	case "storage":
		switch gvk.Kind {
		case "StorageClass":
			return true
		}
	case "":
		switch gvk.Kind {
		case "ConfigMap", "Endpoint", "Event", "LimitRange", "Namespace", "Node",
			"PersistentVolume", "PersistentVolumeClaim", "Pod", "PodTemplate",
			"ReplicationController", "ResourceQuota", "Secret", "Service",
			"ServiceAccount", "EndpointSlice":
			return true
		}
	}

	return false
}

func allowsCreateOnUpdate(gvk schema.GroupVersionKind) bool {
	switch gvk.Group {
	case "coordination":
		switch gvk.Kind {
		case "Lease":
			return true
		}
	case "node":
		switch gvk.Kind {
		case "RuntimeClass":
			return true
		}
	case "rbac":
		switch gvk.Kind {
		case "ClusterRole", "ClusterRoleBinding", "Role", "RoleBinding":
			return true
		}
	case "":
		switch gvk.Kind {
		case "Endpoint", "Event", "LimitRange", "Service":
			return true
		}
	}

	return false
}
