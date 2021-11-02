package tiltfile

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/controllers/apiset"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var (
	apiGVStr = v1alpha1.SchemeGroupVersion.String()
	apiKind  = "Tiltfile"
	apiType  = metav1.TypeMeta{Kind: apiKind, APIVersion: apiGVStr}
)

type disableSourceMap map[model.ManifestName]*v1alpha1.DisableSource

// Update all the objects in the apiserver that are owned by the Tiltfile.
//
// Here we have one big API object (the Tiltfile loader) create lots of
// API objects of different types. This is not a common pattern in Kubernetes-land
// (where often each type will only own one or two other types). But it's the best way
// to model how the Tiltfile works.
//
// For that reason, this code is much more generic than owned-object creation should be.
//
// In the future, anything that creates objects based on the Tiltfile (e.g., FileWatch specs,
// LocalServer specs) should go here.
func updateOwnedObjects(ctx context.Context, client ctrlclient.Client, nn types.NamespacedName,
	tf *v1alpha1.Tiltfile, tlr *tiltfile.TiltfileLoadResult, mode store.EngineMode) error {

	// Assemble the LiveUpdate selectors, connecting objects together.
	if tlr != nil {
		for _, m := range tlr.Manifests {
			err := m.InferLiveUpdateSelectors()
			if err != nil {
				return err
			}
		}
	}

	apiObjects := toAPIObjects(nn, tf, tlr, mode)

	// Propagate labels and owner references from the parent tiltfile.
	for _, objMap := range apiObjects {
		for _, obj := range objMap {
			err := controllerutil.SetControllerReference(tf, obj, client.Scheme())
			if err != nil {
				return err
			}
			propagateLabels(tf, obj)
			propagateAnnotations(tf, obj)
		}
	}

	// Retry until the cache has started.
	var retryCount = 0
	var existingObjects apiset.ObjectSet
	var err error
	for {
		existingObjects, err = getExistingAPIObjects(ctx, client, nn)
		if err != nil {
			if _, ok := err.(*cache.ErrCacheNotStarted); ok && retryCount < 5 {
				retryCount++
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return err
		}
		break
	}

	err = updateNewObjects(ctx, client, apiObjects, existingObjects)
	if err != nil {
		return err
	}

	// If the tiltfile loader succeeded or if the tiltfile was deleted,
	// garbage collect any old objects.
	//
	// If the tiltfile loader failed, we want to keep those objects around, in case
	// the tiltfile was only partially evaluated and is missing objects.
	if tlr == nil || tlr.Error == nil {
		err := removeOrphanedObjects(ctx, client, apiObjects, existingObjects)
		if err != nil {
			return err
		}
	}
	return nil
}

// Apply labels from the Tiltfile to all objects it creates.
func propagateLabels(tf *v1alpha1.Tiltfile, obj apiset.Object) {
	if len(tf.Spec.Labels) > 0 {
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		for k, v := range tf.Spec.Labels {
			// Labels specified during tiltfile execution take precedence over
			// labels specified in the tiltfile spec.
			_, exists := labels[k]
			if !exists {
				labels[k] = v
			}
		}
		obj.SetLabels(labels)
	}
}

// We don't have a great strategy right now for assigning
// API object spec definitions to Manifests in the Tilt UI.
//
// For now, if an object doesn't have a Manifest annotation
// defined, we give it the same Manifest as the parent Tiltfile.
func propagateAnnotations(tf *v1alpha1.Tiltfile, obj apiset.Object) {
	annos := obj.GetAnnotations()
	if annos[v1alpha1.AnnotationManifest] == "" {
		if annos == nil {
			annos = make(map[string]string)
		}
		annos[v1alpha1.AnnotationManifest] = tf.Name
		obj.SetAnnotations(annos)
	}
}

var typesWithTiltfileBuiltins = []apiset.Object{
	&v1alpha1.ExtensionRepo{},
	&v1alpha1.Extension{},
	&v1alpha1.FileWatch{},
	&v1alpha1.Cmd{},
	&v1alpha1.KubernetesApply{},
	&v1alpha1.UIButton{},
	&v1alpha1.ConfigMap{},
	&v1alpha1.KubernetesDiscovery{},
}

var typesToReconcile = append([]apiset.Object{
	&v1alpha1.ImageMap{},
	&v1alpha1.UIResource{},
	&v1alpha1.LiveUpdate{},
}, typesWithTiltfileBuiltins...)

// Fetch all the existing API objects that were generated from the Tiltfile.
func getExistingAPIObjects(ctx context.Context, client ctrlclient.Client, nn types.NamespacedName) (apiset.ObjectSet, error) {
	result := apiset.ObjectSet{}

	// TODO(nick): Parallelize this?
	for _, obj := range typesToReconcile {
		list := obj.NewList().(ctrlclient.ObjectList)
		err := indexer.ListOwnedBy(ctx, client, list, nn, apiType)
		if err != nil {
			return nil, err
		}

		_ = meta.EachListItem(list, func(obj runtime.Object) error {
			result.Add(obj.(apiset.Object))
			return nil
		})
	}

	return result, nil
}

// Pulls out all the API objects generated by the Tiltfile.
func toAPIObjects(nn types.NamespacedName, tf *v1alpha1.Tiltfile, tlr *tiltfile.TiltfileLoadResult, mode store.EngineMode) apiset.ObjectSet {
	result := apiset.ObjectSet{}

	var disableSources disableSourceMap

	if tlr != nil {
		disableSources = toDisableSources(tlr)
		result.AddSetForType(&v1alpha1.ImageMap{}, toImageMapObjects(tlr, disableSources))
		result.AddSetForType(&v1alpha1.LiveUpdate{}, toLiveUpdateObjects(tlr))

		for _, obj := range typesWithTiltfileBuiltins {
			result.AddSetForType(obj, tlr.ObjectSet.GetSetForType(obj))
		}

		kaMap := result.GetOrCreateTypedSet(&v1alpha1.KubernetesApply{})
		for k, obj := range toKubernetesApplyObjects(tlr, disableSources) {
			kaMap[k] = obj
		}

		cmMap := result.GetOrCreateTypedSet(&v1alpha1.ConfigMap{})
		for k, obj := range toDisableConfigMaps(disableSources) {
			cmMap[k] = obj
		}

		updateCmds := toCmdObjects(tlr, disableSources)
		cmdMap := result.GetOrCreateTypedSet(&v1alpha1.Cmd{})
		for key, cmd := range updateCmds {
			cmdMap[key] = cmd
		}

		result.AddSetForType(&v1alpha1.ToggleButton{}, toToggleButtons(tlr, disableSources))
	} else {
		disableSources = make(disableSourceMap)
	}

	result.AddSetForType(&v1alpha1.UIResource{}, toUIResourceObjects(tf, tlr, disableSources))

	watchInputs := WatchInputs{
		TiltfileManifestName: model.ManifestName(nn.Name),
		EngineMode:           mode,
	}

	if tlr != nil {
		watchInputs.Manifests = tlr.Manifests
		watchInputs.ConfigFiles = tlr.ConfigFiles
		watchInputs.Tiltignore = tlr.Tiltignore
		watchInputs.WatchSettings = tlr.WatchSettings
	}

	if tf != nil {
		watchInputs.TiltfilePath = tf.Spec.Path
	}

	fwMap := result.GetOrCreateTypedSet(&v1alpha1.FileWatch{})
	for k, fw := range ToFileWatchObjects(watchInputs, disableSources) {
		fwMap[k] = fw
	}

	return result
}

func disableConfigMapName(manifest model.Manifest) string {
	return fmt.Sprintf("%s-disable", manifest.Name)
}

func toDisableSources(tlr *tiltfile.TiltfileLoadResult) disableSourceMap {
	result := make(disableSourceMap)
	for _, m := range tlr.Manifests {
		name := disableConfigMapName(m)
		ds := &v1alpha1.DisableSource{
			ConfigMap: &v1alpha1.ConfigMapDisableSource{
				Name: name,
				Key:  "isDisabled",
			},
		}
		result[m.Name] = ds
	}
	return result
}

func toDisableConfigMaps(disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	for _, ds := range disableSources {
		cm := &v1alpha1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: ds.ConfigMap.Name,
			},
			Data: map[string]string{ds.ConfigMap.Key: "false"},
		}
		result[cm.Name] = cm
	}
	return result
}

func toToggleButtons(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	if tlr != nil && tlr.FeatureFlags[feature.DisableResources] {
		for name, ds := range disableSources {
			// TODO(matt) - add/set a field to make sure this displays in the right location
			// https://app.shortcut.com/windmill/story/12866/backend-creates-enable-disable-togglebuttons
			tb := &v1alpha1.ToggleButton{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-disable", name),
				},
				Spec: v1alpha1.ToggleButtonSpec{
					Location: v1alpha1.UIComponentLocation{
						ComponentID:   string(name),
						ComponentType: v1alpha1.ComponentTypeResource,
					},
					On: v1alpha1.ToggleButtonStateSpec{
						Text:     "Enable",
						IconName: "play_arrow",
					},
					Off: v1alpha1.ToggleButtonStateSpec{
						Text:                 "Disable",
						IconName:             "stop",
						RequiresConfirmation: true,
					},
					StateSource: v1alpha1.StateSource{
						ConfigMap: &v1alpha1.ConfigMapStateSource{
							Name:     ds.ConfigMap.Name,
							Key:      ds.ConfigMap.Key,
							OnValue:  "true",
							OffValue: "false",
						},
					},
					ButtonType: v1alpha1.UIButtonTypeDisableToggle,
				},
			}
			result[tb.Name] = tb
		}
	}
	return result
}

// Pulls out all the KubernetesApply objects generated by the Tiltfile.
func toKubernetesApplyObjects(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	for _, m := range tlr.Manifests {
		if !m.IsK8s() {
			continue
		}

		kTarget := m.K8sTarget()
		name := m.Name.String()
		ka := &v1alpha1.KubernetesApply{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					v1alpha1.AnnotationManifest:  name,
					v1alpha1.AnnotationSpanID:    fmt.Sprintf("kubernetesapply:%s", name),
					v1alpha1.AnnotationManagedBy: "buildcontrol",
				},
			},
			Spec: kTarget.KubernetesApplySpec,
		}
		ka.Spec.DisableSource = disableSources[m.Name]
		result[name] = ka
	}
	return result
}

// Pulls out all the LiveUpdate objects generated by the Tiltfile.
func toLiveUpdateObjects(tlr *tiltfile.TiltfileLoadResult) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	for _, m := range tlr.Manifests {
		for _, iTarget := range m.ImageTargets {
			luSpec := iTarget.LiveUpdateSpec
			luName := iTarget.LiveUpdateName
			if liveupdate.IsEmptySpec(luSpec) || luName == "" {
				continue
			}

			managedBy := ""
			if !iTarget.LiveUpdateReconciler {
				managedBy = "buildcontrol"
			}

			updateMode := liveupdate.UpdateModeAuto
			if !m.TriggerMode.AutoOnChange() {
				updateMode = liveupdate.UpdateModeManual
			}

			obj := &v1alpha1.LiveUpdate{
				ObjectMeta: metav1.ObjectMeta{
					Name: luName,
					Annotations: map[string]string{
						v1alpha1.AnnotationManifest:     m.Name.String(),
						v1alpha1.AnnotationSpanID:       fmt.Sprintf("liveupdate:%s", luName),
						v1alpha1.AnnotationManagedBy:    managedBy,
						liveupdate.AnnotationUpdateMode: updateMode,
					},
				},
				Spec: luSpec,
			}
			result[luName] = obj
		}
	}
	return result
}

// Pulls out all the ImageMap objects generated by the Tiltfile.
func toImageMapObjects(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}

	for _, m := range tlr.Manifests {
		for _, iTarget := range m.ImageTargets {
			if iTarget.IsLiveUpdateOnly {
				continue
			}

			name := apis.SanitizeName(iTarget.ID().Name.String())
			// Note that an ImageMap might be in more than one Manifest, so we
			// can't annotate them to a particular manifest.
			im := &v1alpha1.ImageMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Annotations: map[string]string{
						v1alpha1.AnnotationManifest: m.Name.String(),
						v1alpha1.AnnotationSpanID:   fmt.Sprintf("imagemap:%s", name),
					},
				},
				Spec: iTarget.ImageMapSpec,
			}
			im.Spec.DisableSource = disableSources[m.Name]
			result[name] = im
		}
	}
	return result
}

// Pulls out all the Cmd objects generated by the Tiltfile.
func toCmdObjects(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}

	for _, m := range tlr.Manifests {
		if !m.IsLocal() {
			continue
		}
		localTarget := m.LocalTarget()
		cmdSpec := localTarget.UpdateCmdSpec
		if cmdSpec == nil {
			continue
		}

		name := localTarget.UpdateCmdName()
		cmd := &v1alpha1.Cmd{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					v1alpha1.AnnotationManifest:  m.Name.String(),
					v1alpha1.AnnotationSpanID:    fmt.Sprintf("cmd:%s", name),
					v1alpha1.AnnotationManagedBy: "local_resource",
				},
			},
			Spec: *cmdSpec,
		}
		cmd.Spec.DisableSource = disableSources[m.Name]
		result[name] = cmd
	}
	return result
}

// Pulls out all the UIResource objects generated by the Tiltfile.
func toUIResourceObjects(tf *v1alpha1.Tiltfile, tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}

	if tlr != nil {
		for _, m := range tlr.Manifests {
			name := string(m.Name)

			r := &v1alpha1.UIResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: m.Labels,
					Annotations: map[string]string{
						v1alpha1.AnnotationManifest: m.Name.String(),
					},
				},
			}

			ds := disableSources[m.Name]
			if ds != nil {
				r.Status.DisableStatus.Sources = []v1alpha1.DisableSource{*ds}
			}

			result[name] = r
		}
	}

	if tf != nil {
		result[tf.Name] = &v1alpha1.UIResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:   tf.Name,
				Labels: tf.Labels,
				Annotations: map[string]string{
					v1alpha1.AnnotationManifest: tf.Name,
				},
			},
		}
	}

	return result
}

// Reconcile the new API objects against the existing API objects.
func updateNewObjects(ctx context.Context, client ctrlclient.Client, newObjects, oldObjects apiset.ObjectSet) error {
	// TODO(nick): Does it make sense to parallelize the API calls?
	errs := []error{}

	// Upsert the new objects.
	for t, s := range newObjects {
		for name, obj := range s {
			var old apiset.Object
			oldSet, ok := oldObjects[t]
			if ok {
				old = oldSet[name]
			}

			if old == nil {
				err := client.Create(ctx, obj)
				if err != nil {
					errs = append(errs, fmt.Errorf("create %s/%s: %v", obj.GetGroupVersionResource().Resource, obj.GetName(), err))
				}
				continue
			}

			// Are there other fields here we should check?
			specChanged := !apicmp.DeepEqual(old.GetSpec(), obj.GetSpec())
			labelsChanged := !apicmp.DeepEqual(old.GetLabels(), obj.GetLabels())
			annsChanged := !apicmp.DeepEqual(old.GetAnnotations(), obj.GetAnnotations())
			if specChanged || labelsChanged || annsChanged {
				obj.SetResourceVersion(old.GetResourceVersion())
				if cm, ok := obj.(*v1alpha1.ConfigMap); ok {
					// Tiltfiles can create ConfigMaps with default values, but
					// they shouldn't blow away values that were modified elsewhere.
					// At some point, we might have a use case that contradicts this and
					// need to make this more complicated.
					for k, v := range old.(*v1alpha1.ConfigMap).Data {
						cm.Data[k] = v
					}
				}
				err := client.Update(ctx, obj)
				if err != nil {
					errs = append(errs, fmt.Errorf("update %s/%s: %v", obj.GetGroupVersionResource().Resource, obj.GetName(), err))
				}
				continue
			}
		}
	}
	return errors.NewAggregate(errs)
}

// Garbage collect API objects that are no longer loaded.
func removeOrphanedObjects(ctx context.Context, client ctrlclient.Client, newObjects, oldObjects apiset.ObjectSet) error {
	// Delete any objects that aren't in the new tiltfile.
	errs := []error{}
	for t, s := range oldObjects {
		for name, obj := range s {
			newSet, ok := newObjects[t]
			if ok {
				_, ok := newSet[name]
				if ok {
					continue
				}
			}

			err := client.Delete(ctx, obj)
			if err != nil {
				errs = append(errs, fmt.Errorf("delete %s/%s: %v", obj.GetGroupVersionResource().Resource, obj.GetName(), err))
			}
		}
	}
	return errors.NewAggregate(errs)
}
