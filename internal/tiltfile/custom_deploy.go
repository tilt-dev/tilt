package tiltfile

import (
	"fmt"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

type k8sCustomDeploy struct {
	cmd  model.Cmd
	deps []string
}

func (s *tiltfileState) k8sCustomDeploy(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !s.features.Get(feature.K8sCustomDeploy) {
		return nil, errors.New("k8s_custom_deploy is not supported by this version of Tilt")
	}

	var name string
	var cmdVal, cmdBatVal, cmdDirVal starlark.Value
	var cmdEnv value.StringStringMap
	var imageSelector string
	var liveUpdateVal starlark.Value

	deps := value.NewLocalPathListUnpacker(thread)

	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"name", &name,
		"cmd", &cmdVal,
		"deps", &deps,
		"image_selector?", &imageSelector,
		"live_update?", &liveUpdateVal,
		"dir?", &cmdDirVal,
		"env?", &cmdEnv,
		"cmd_bat?", &cmdBatVal,
	); err != nil {
		return nil, err
	}

	cmd, err := value.ValueGroupToCmdHelper(thread, cmdVal, cmdBatVal, cmdDirVal, cmdEnv)
	if err != nil {
		return nil, errors.Wrap(err, "cmd")
	} else if cmd.Empty() {
		return nil, fmt.Errorf("k8s_custom_deploy: cmd cannot be empty")
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
		cmd:  cmd,
		deps: deps.Value,
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
