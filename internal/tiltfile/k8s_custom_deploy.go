package tiltfile

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type k8sCustomDeploy struct {
	applyCmd  model.Cmd
	deleteCmd model.Cmd
	deps      []string
}

func (s *tiltfileState) k8sCustomDeploy(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var applyCmdVal, applyCmdBatVal, applyCmdDirVal starlark.Value
	var deleteCmdVal, deleteCmdBatVal, deleteCmdDirVal starlark.Value
	var applyCmdEnv, deleteCmdEnv value.StringStringMap
	var imageSelector, containerSelector string
	var liveUpdateVal starlark.Value
	var imageDeps value.ImageList

	deps := value.NewLocalPathListUnpacker(thread)

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"apply_cmd", &applyCmdVal,
		"delete_cmd", &deleteCmdVal,
		"deps", &deps,
		"image_selector?", &imageSelector,
		"live_update?", &liveUpdateVal,
		"apply_dir?", &applyCmdDirVal,
		"apply_env?", &applyCmdEnv,
		"apply_cmd_bat?", &applyCmdBatVal,
		"delete_dir?", &deleteCmdDirVal,
		"delete_env?", &deleteCmdEnv,
		"delete_cmd_bat?", &deleteCmdBatVal,
		"container_selector?", &containerSelector,
		"image_deps?", &imageDeps,
	); err != nil {
		return nil, err
	}

	applyCmd, err := value.ValueGroupToCmdHelper(thread, applyCmdVal, applyCmdBatVal, applyCmdDirVal, applyCmdEnv)
	if err != nil {
		return nil, errors.Wrap(err, "apply_cmd")
	} else if applyCmd.Empty() {
		return nil, fmt.Errorf("k8s_custom_deploy: apply_cmd cannot be empty")
	}

	deleteCmd, err := value.ValueGroupToCmdHelper(thread, deleteCmdVal, deleteCmdBatVal, deleteCmdDirVal, deleteCmdEnv)
	if err != nil {
		return nil, errors.Wrap(err, "delete_cmd")
	} else if deleteCmd.Empty() {
		return nil, fmt.Errorf("k8s_custom_deploy: delete_cmd cannot be empty")
	}

	liveUpdate, err := s.liveUpdateFromSteps(thread, liveUpdateVal)
	if err != nil {
		return nil, errors.Wrap(err, "live_update")
	}

	res, err := s.makeK8sResource(name)
	if err != nil {
		return nil, fmt.Errorf("error making resource for %s: %v", name, err)
	}

	deployDeps, luDeps := filterDepsForCustomDeploy(deps.Value, liveUpdate)

	res.customDeploy = &k8sCustomDeploy{
		applyCmd:  applyCmd,
		deleteCmd: deleteCmd,
		deps:      deployDeps,
	}
	for _, imageDep := range imageDeps {
		res.addImageDep(imageDep, true)
	}

	if !liveupdate.IsEmptySpec(liveUpdate) {
		var ref reference.Named
		var selectorCount int

		if imageSelector != "" {
			selectorCount++

			// the ref attached to the image target will be inferred as the image selector
			// for the LiveUpdateSpec by Manifest::InferLiveUpdateSelectors
			ref, err = container.ParseNamed(imageSelector)
			if err != nil {
				return nil, fmt.Errorf("can't parse %q: %v", imageSelector, err)
			}
		}

		if containerSelector != "" {
			selectorCount++

			// pre-populate the container name selector as this cannot be inferred from
			// the image target by Manifest::InferLiveUpdateSelectors
			liveUpdate.Selector.Kubernetes = &v1alpha1.LiveUpdateKubernetesSelector{
				ContainerName: containerSelector,
			}

			// the image target needs a valid ref even though it'll never be
			// built/used, so create one named after the manifest that won't
			// collide with anything else
			fakeImageName := fmt.Sprintf("k8s_custom_deploy:%s", name)
			ref, err = container.ParseNamed(fakeImageName)
			if err != nil {
				return nil, fmt.Errorf("can't parse %q: %v", fakeImageName, err)
			}
		}

		if selectorCount == 0 {
			return nil, fmt.Errorf("k8s_custom_deploy: no Live Update selector specified")
		} else if selectorCount > 1 {
			return nil, fmt.Errorf("k8s_custom_deploy: cannot specify more than one Live Update selector")
		}

		img := &dockerImage{
			buildType:        CustomBuild,
			configurationRef: container.NewRefSelector(ref),
			// HACK(milas): this is treated specially in the BuildAndDeployer to
			// 	mark this as a "LiveUpdateOnly" ImageTarget, so that no builds
			// 	will be done, only deploy + Live Update
			customCommand:    model.ToHostCmd(":"),
			customDeps:       luDeps,
			liveUpdate:       liveUpdate,
			disablePush:      true,
			skipsLocalDocker: true,
			tiltfilePath:     starkit.CurrentExecPath(thread),
		}
		// N.B. even in the case that we're creating a fake image name, we need
		// 	to reference it so that it can be "consumed" by this target to avoid
		// 	producing warnings about unused image targets
		res.addImageDep(ref, false)

		if err := s.buildIndex.addImage(img); err != nil {
			return nil, err
		}
	}

	return starlark.None, nil
}

func filterDepsForCustomDeploy(deps []string, spec v1alpha1.LiveUpdateSpec) (deployPaths []string, luPaths []string) {
	luMatcher := pathMatcherFromLiveUpdateSpec(spec)
	for _, dep := range deps {
		if match, _ := luMatcher.Matches(dep); match {
			luPaths = append(luPaths, dep)
		} else {
			deployPaths = append(deployPaths, dep)
		}
	}
	return
}

func pathMatcherFromLiveUpdateSpec(spec v1alpha1.LiveUpdateSpec) model.PathMatcher {
	paths := make([]string, 0, len(spec.Syncs)+len(spec.StopPaths))
	for _, sync := range spec.Syncs {
		paths = append(paths, sync.LocalPath)
	}
	paths = append(paths, spec.StopPaths...)
	return model.NewRelativeFileOrChildMatcher(spec.BasePath, paths...)
}
