package tiltfile

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-cmp/cmp"
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
	"github.com/tilt-dev/tilt/internal/controllers/apis/uibutton"
	"github.com/tilt-dev/tilt/internal/controllers/apiset"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/sessions"
	"github.com/tilt-dev/tilt/internal/tiltfile"
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
func updateOwnedObjects(
	ctx context.Context,
	client ctrlclient.Client,
	nn types.NamespacedName,
	tf *v1alpha1.Tiltfile,
	tlr *tiltfile.TiltfileLoadResult,
	changeEnabledResources bool,
	ciTimeoutFlag model.CITimeoutFlag,
	mode store.EngineMode,
	defaultK8sConnection *v1alpha1.KubernetesClusterConnection,
) error {
	disableSources := toDisableSources(tlr)

	if tlr != nil {
		// Apply the registry to the image refs.
		err := model.InferImageProperties(tlr.Manifests)
		if err != nil {
			return err
		}

		for i, m := range tlr.Manifests {
			// Assemble the LiveUpdate selectors, connecting objects together.
			err = m.InferLiveUpdateSelectors()
			if err != nil {
				return err
			}

			tlr.Manifests[i] = m.WithDisableSource(disableSources[m.Name])
		}
	}

	apiObjects := toAPIObjects(nn, tf, tlr, ciTimeoutFlag, mode, defaultK8sConnection, disableSources)

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

	if !changeEnabledResources {
		// if we're not changing enabled resources, use existing values for disable configmaps
		newConfigMaps := apiObjects.GetSetForType(&v1alpha1.ConfigMap{})
		oldConfigMaps := existingObjects.GetSetForType(&v1alpha1.ConfigMap{})
		for _, ds := range disableSources {
			if old, ok := oldConfigMaps[ds.ConfigMap.Name]; ok {
				newConfigMaps[ds.ConfigMap.Name] = old
			}
		}
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
	&v1alpha1.CmdImage{},
	&v1alpha1.DockerImage{},
	&v1alpha1.UIResource{},
	&v1alpha1.LiveUpdate{},
	&v1alpha1.ToggleButton{},
	&v1alpha1.Cluster{},
	&v1alpha1.DockerComposeService{},
	&v1alpha1.Session{},
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
func toAPIObjects(
	nn types.NamespacedName,
	tf *v1alpha1.Tiltfile,
	tlr *tiltfile.TiltfileLoadResult,
	ciTimeoutFlag model.CITimeoutFlag,
	mode store.EngineMode,
	defaultK8sConnection *v1alpha1.KubernetesClusterConnection,
	disableSources disableSourceMap,
) apiset.ObjectSet {
	result := apiset.ObjectSet{}

	if tlr != nil {
		result.AddSetForType(&v1alpha1.ImageMap{}, toImageMapObjects(tlr, disableSources))
		result.AddSetForType(&v1alpha1.LiveUpdate{}, toLiveUpdateObjects(tlr))
		result.AddSetForType(&v1alpha1.DockerImage{}, toDockerImageObjects(tlr, disableSources))
		result.AddSetForType(&v1alpha1.CmdImage{}, toCmdImageObjects(tlr, disableSources))

		for _, obj := range typesWithTiltfileBuiltins {
			result.AddSetForType(obj, tlr.ObjectSet.GetSetForType(obj))
		}

		result.AddSetForType(&v1alpha1.KubernetesApply{}, toKubernetesApplyObjects(tlr, disableSources))
		result.AddSetForType(&v1alpha1.DockerComposeService{}, toDockerComposeServiceObjects(tlr, disableSources))
		result.AddSetForType(&v1alpha1.ConfigMap{}, toDisableConfigMaps(disableSources, tlr.EnabledManifests))
		result.AddSetForType(&v1alpha1.Cmd{}, toCmdObjects(tlr, disableSources))
		result.AddSetForType(&v1alpha1.ToggleButton{}, toToggleButtons(disableSources))
		result.AddSetForType(&v1alpha1.Cluster{}, toClusterObjects(nn, tlr, defaultK8sConnection))
		result.AddSetForType(&v1alpha1.UIButton{}, toCancelButtons(tlr))
	}

	result.AddSetForType(&v1alpha1.Session{}, toSessionObjects(nn, tf, tlr, ciTimeoutFlag, mode))
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

	result.AddSetForType(&v1alpha1.FileWatch{}, ToFileWatchObjects(watchInputs, disableSources))

	return result
}

func disableConfigMapName(manifest model.Manifest) string {
	return fmt.Sprintf("%s-disable", manifest.Name)
}

func toDisableSources(tlr *tiltfile.TiltfileLoadResult) disableSourceMap {
	result := make(disableSourceMap)
	if tlr != nil {
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
	}
	return result
}

func appendCMDS(cms []v1alpha1.ConfigMapDisableSource, newCM v1alpha1.ConfigMapDisableSource) []v1alpha1.ConfigMapDisableSource {
	for _, cm := range cms {
		if apicmp.DeepEqual(cm, newCM) {
			return cms
		}
	}
	return append(cms, newCM)
}

func mergeDisableSource(existing *v1alpha1.DisableSource, toMerge *v1alpha1.DisableSource) *v1alpha1.DisableSource {
	if toMerge == nil {
		return existing
	}
	if apicmp.DeepEqual(existing, toMerge) {
		return existing
	}

	cms := []v1alpha1.ConfigMapDisableSource{}

	if existing.ConfigMap != nil {
		cms = append(cms, *existing.ConfigMap)
	}
	cms = append(cms, existing.EveryConfigMap...)
	if toMerge.ConfigMap != nil {
		cms = appendCMDS(cms, *toMerge.ConfigMap)
	}
	for _, newCM := range toMerge.EveryConfigMap {
		cms = appendCMDS(cms, newCM)
	}
	return &v1alpha1.DisableSource{EveryConfigMap: cms}
}

func toDisableConfigMaps(disableSources disableSourceMap, enabledResources []model.ManifestName) apiset.TypedObjectSet {
	enabledResourceSet := make(map[model.ManifestName]bool)
	for _, mn := range enabledResources {
		enabledResourceSet[mn] = true
	}
	result := apiset.TypedObjectSet{}
	for mn, ds := range disableSources {
		isDisabled := !enabledResourceSet[mn]
		cm := &v1alpha1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: ds.ConfigMap.Name,
			},
			Data: map[string]string{ds.ConfigMap.Key: strconv.FormatBool(isDisabled)},
		}
		result[cm.Name] = cm
	}
	return result
}

func toToggleButtons(disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	for name, ds := range disableSources {
		tb := &v1alpha1.ToggleButton{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-disable", name),
				Annotations: map[string]string{
					v1alpha1.AnnotationButtonType: v1alpha1.ButtonTypeDisableToggle,
				},
			},
			Spec: v1alpha1.ToggleButtonSpec{
				Location: v1alpha1.UIComponentLocation{
					ComponentID:   string(name),
					ComponentType: v1alpha1.ComponentTypeResource,
				},
				On: v1alpha1.ToggleButtonStateSpec{
					Text: "Enable Resource",
				},
				Off: v1alpha1.ToggleButtonStateSpec{
					Text:                 "Disable Resource",
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
			},
		}
		result[tb.Name] = tb
	}
	return result
}

func toCancelButtons(tlr *tiltfile.TiltfileLoadResult) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	for _, m := range tlr.Manifests {
		button := uibutton.StopBuildButton(m.Name.String())
		result[button.Name] = button
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

// Pulls out all the DockerComposeService objects generated by the Tiltfile.
func toDockerComposeServiceObjects(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	for _, m := range tlr.Manifests {
		if !m.IsDC() {
			continue
		}

		dcTarget := m.DockerComposeTarget()
		name := m.Name.String()
		obj := &v1alpha1.DockerComposeService{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					v1alpha1.AnnotationManifest:  name,
					v1alpha1.AnnotationSpanID:    fmt.Sprintf("dockercompose:%s", name),
					v1alpha1.AnnotationManagedBy: "buildcontrol",
				},
			},
			Spec: dcTarget.Spec,
		}
		obj.Spec.DisableSource = disableSources[m.Name]
		result[name] = obj
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

// Pulls out all the DockerImage objects generated by the Tiltfile.
func toDockerImageObjects(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}

	for _, m := range tlr.Manifests {
		for _, iTarget := range m.ImageTargets {
			name := iTarget.DockerImageName
			if name == "" {
				continue
			}

			// Currently, if a DockerImage is in more than one manifest,
			// we will create one per manifest.
			//
			// In the medium-term, we should try to annotate them in a way
			// that allows manifests to share images.
			di := &v1alpha1.DockerImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Annotations: map[string]string{
						v1alpha1.AnnotationManifest: m.Name.String(),
						v1alpha1.AnnotationSpanID:   fmt.Sprintf("dockerimage:%s", name),
					},
				},
				Spec: iTarget.DockerBuildInfo().DockerImageSpec,
			}

			// TODO(nick): Add DisableSource to image builds.
			//di.Spec.DisableSource = disableSources[m.Name]

			result[name] = di
		}
	}
	return result
}

// Pulls out all the CmdImage objects generated by the Tiltfile.
func toCmdImageObjects(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}

	for _, m := range tlr.Manifests {
		for _, iTarget := range m.ImageTargets {
			name := iTarget.CmdImageName
			if name == "" {
				continue
			}

			// Currently, if a CmdImage is in more than one manifest,
			// we will create one per manifest.
			//
			// In the medium-term, we should try to annotate them in a way
			// that allows manifests to share images.
			ci := &v1alpha1.CmdImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Annotations: map[string]string{
						v1alpha1.AnnotationManifest: m.Name.String(),
						v1alpha1.AnnotationSpanID:   fmt.Sprintf("cmdimage:%s", name),
					},
				},
				Spec: iTarget.CustomBuildInfo().CmdImageSpec,
			}

			// TODO(nick): Add DisableSource to image builds.
			// di.Spec.DisableSource = disableSources[m.Name]

			result[name] = ci
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

			name := iTarget.ImageMapName()
			_, ok := result[name]
			if ok {
				// Some ImageTargets are shared among multiple manifests.
				continue
			}

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
			result[name] = im
		}
	}
	return result
}

// Creates an object representing the tilt session and exit conditions.
func toSessionObjects(nn types.NamespacedName, tf *v1alpha1.Tiltfile, tlr *tiltfile.TiltfileLoadResult, ciTimeoutFlag model.CITimeoutFlag, mode store.EngineMode) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	if nn.Name != model.MainTiltfileManifestName.String() {
		return result
	}
	result[sessions.DefaultSessionName] = sessions.FromTiltfile(tf, tlr, ciTimeoutFlag, mode)
	return result
}

// Pulls out any cluster objects generated by the tiltfile.
func toClusterObjects(nn types.NamespacedName, tlr *tiltfile.TiltfileLoadResult, defaultK8sConnection *v1alpha1.KubernetesClusterConnection) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}
	if nn.Name != model.MainTiltfileManifestName.String() {
		return result
	}

	var annotations map[string]string
	if tlr.FeatureFlags[feature.ClusterRefresh] {
		annotations = map[string]string{
			"features.tilt.dev/cluster-refresh": "true",
		}
	}

	if tlr.HasOrchestrator(model.OrchestratorK8s) {
		name := v1alpha1.ClusterNameDefault
		result[name] = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Annotations: annotations,
			},
			Spec: v1alpha1.ClusterSpec{
				Connection: &v1alpha1.ClusterConnection{
					Kubernetes: defaultK8sConnection.DeepCopy(),
				},
				DefaultRegistry: tlr.DefaultRegistry,
			},
		}
	}

	if tlr.HasOrchestrator(model.OrchestratorDC) {
		name := v1alpha1.ClusterNameDocker
		result[name] = &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Annotations: annotations,
			},
			Spec: v1alpha1.ClusterSpec{
				Connection: &v1alpha1.ClusterConnection{
					Docker: &v1alpha1.DockerClusterConnection{},
				},
			},
		}
	}

	return result
}

// Pulls out all the Cmd objects generated by the Tiltfile.
func toCmdObjects(tlr *tiltfile.TiltfileLoadResult, disableSources disableSourceMap) apiset.TypedObjectSet {
	result := apiset.TypedObjectSet{}

	// Every local_resource's Update Cmd gets its own object.
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

	// Every custom_build Cmd gets its own Cmd object.
	// It would be better for the CmdImage reconciler to create these
	// and make them owned by the CmdImage.
	for _, m := range tlr.Manifests {
		for _, iTarget := range m.ImageTargets {
			name := iTarget.CmdImageName
			if name == "" {
				continue
			}

			// Currently, if a CmdImage is in more than one manifest,
			// we will create one per manifest.
			//
			// In the medium-term, we should try to annotate them in a way
			// that allows manifests to share images.
			cmdimageSpec := iTarget.CustomBuildInfo().CmdImageSpec
			cmd := &v1alpha1.Cmd{
				ObjectMeta: metav1.ObjectMeta{
					Name: iTarget.CmdImageName,
					Annotations: map[string]string{
						v1alpha1.AnnotationManifest:  m.Name.String(),
						v1alpha1.AnnotationSpanID:    fmt.Sprintf("cmdimage:%s", name),
						v1alpha1.AnnotationManagedBy: "cmd_image",
					},
				},
				Spec: v1alpha1.CmdSpec{
					Args: cmdimageSpec.Args,
					Dir:  cmdimageSpec.Dir,
				},
			}

			// TODO(nick): Add DisableSource to image builds.
			// cmd.Spec.DisableSource = disableSources[m.Name]

			result[name] = cmd
		}
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
				r.Status.DisableStatus.State = v1alpha1.DisableStatePending
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

func needsUpdate(old, obj apiset.Object) bool {
	// Are there other fields here we should check?
	specChanged := !apicmp.DeepEqual(old.GetSpec(), obj.GetSpec())
	labelsChanged := !apicmp.DeepEqual(old.GetLabels(), obj.GetLabels())
	annsChanged := !apicmp.DeepEqual(old.GetAnnotations(), obj.GetAnnotations())
	if specChanged || labelsChanged || annsChanged {
		return true
	}

	// if this section ends up with more type-specific checks, we should probably move this
	// to be a method on apiset.Object
	if cm, ok := obj.(*v1alpha1.ConfigMap); ok {
		if !cmp.Equal(cm.Data, old.(*v1alpha1.ConfigMap).Data) {
			return true
		}
	}

	return false
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

			if needsUpdate(old, obj) {
				obj.SetResourceVersion(old.GetResourceVersion())
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
