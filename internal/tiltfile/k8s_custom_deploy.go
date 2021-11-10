package tiltfile

import (
	"fmt"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
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
	var imageSelector string
	var liveUpdateVal starlark.Value

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

	res.customDeploy = &k8sCustomDeploy{
		applyCmd:  applyCmd,
		deleteCmd: deleteCmd,
		deps:      deps.Value,
	}

	if !liveupdate.IsEmptySpec(liveUpdate) {
		if imageSelector == "" {
			return nil, fmt.Errorf("k8s_custom_deploy: image_selector cannot be empty")
		}

		ref, err := container.ParseNamed(imageSelector)
		if err != nil {
			return nil, fmt.Errorf("can't parse %q: %v", imageSelector, err)
		}

		img := &dockerImage{
			configurationRef: container.NewRefSelector(ref),
			// HACK(milas): this is treated specially in the BuildAndDeployer to
			// 	mark this as a "LiveUpdateOnly" ImageTarget, so that no builds
			// 	will be done, only deploy + Live Update
			customCommand:    model.ToHostCmd(":"),
			customDeps:       deps.Value,
			liveUpdate:       liveUpdate,
			disablePush:      true,
			skipsLocalDocker: true,
			tiltfilePath:     starkit.CurrentExecPath(thread),
		}
		res.imageRefs = append(res.imageRefs, ref)

		if err := s.buildIndex.addImage(img); err != nil {
			return nil, err
		}
	}

	return starlark.None, nil
}
