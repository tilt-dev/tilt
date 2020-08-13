package tiltfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/model"
)

var fastBuildDeletedErr = fmt.Errorf("fast_build is no longer supported. live_update provides the same functionality with less set-up: https://docs.tilt.dev/live_update_tutorial.html . If you run into problems, let us know: https://tilt.dev/contact")
var cacheObsoleteWarning = "docker_build(cache=...) is obsolete, and currently a no-op.\n" +
	"You should switch to live_update to optimize your builds."

type dockerImage struct {
	workDir          string
	configurationRef container.RefSelector
	matchInEnvVars   bool
	sshSpecs         []string
	secretSpecs      []string
	ignores          []string
	onlys            []string
	entrypoint       model.Cmd // optional: if specified, we override the image entrypoint/k8s command with this
	targetStage      string    // optional: if specified, we build a particular target in the dockerfile
	network          string
	extraTags        []string // Extra tags added at build-time.
	cacheFrom        []string
	pullParent       bool

	// Overrides the container args. Used as an escape hatch in case people want the old entrypoint behavior.
	// See discussion here:
	// https://github.com/tilt-dev/tilt/pull/2933
	containerArgs model.OverrideArgs

	dbDockerfilePath string
	dbDockerfile     dockerfile.Dockerfile
	dbBuildPath      string
	dbBuildArgs      model.DockerBuildArgs
	customCommand    model.Cmd
	customDeps       []string
	customTag        string

	// Whether this has been matched up yet to a deploy resource.
	matched bool

	dependencyIDs    []model.TargetID
	disablePush      bool
	skipsLocalDocker bool

	liveUpdate model.LiveUpdate
}

func (d *dockerImage) ID() model.TargetID {
	return model.ImageID(d.configurationRef)
}

type dockerImageBuildType int

const (
	UnknownBuild = iota
	DockerBuild
	CustomBuild
)

func (d *dockerImage) Type() dockerImageBuildType {
	if d.dbBuildPath != "" {
		return DockerBuild
	}

	if !d.customCommand.Empty() {
		return CustomBuild
	}

	return UnknownBuild
}

func (s *tiltfileState) dockerBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dockerRef, targetStage string
	var contextVal,
		dockerfilePathVal,
		buildArgs,
		dockerfileContentsVal,
		cacheVal,
		liveUpdateVal,
		ignoreVal,
		onlyVal,
		entrypoint starlark.Value
	var network value.Stringable
	var ssh, secret, extraTags, cacheFrom value.StringOrStringList
	var matchInEnvVars, pullParent bool
	var containerArgsVal starlark.Sequence
	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"context", &contextVal,
		"build_args?", &buildArgs,
		"dockerfile?", &dockerfilePathVal,
		"dockerfile_contents?", &dockerfileContentsVal,
		"cache?", &cacheVal,
		"live_update?", &liveUpdateVal,
		"match_in_env_vars?", &matchInEnvVars,
		"ignore?", &ignoreVal,
		"only?", &onlyVal,
		"entrypoint?", &entrypoint,
		"container_args?", &containerArgsVal,
		"target?", &targetStage,
		"ssh?", &ssh,
		"secret?", &secret,
		"network?", &network,
		"extra_tag?", &extraTags,
		"cache_from?", &cacheFrom,
		"pull?", &pullParent,
	); err != nil {
		return nil, err
	}

	ref, err := container.ParseNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Argument 1 (ref): can't parse %q: %v", dockerRef, err)
	}

	if contextVal == nil {
		return nil, fmt.Errorf("Argument 2 (context): empty but is required")
	}
	context, err := value.ValueToAbsPath(thread, contextVal)
	if err != nil {
		return nil, err
	}

	sba, err := value.ValueToStringMap(buildArgs)
	if err != nil {
		return nil, fmt.Errorf("Argument 3 (build_args): %v", err)
	}

	dockerfilePath := filepath.Join(context, "Dockerfile")
	var dockerfileContents string
	if dockerfileContentsVal != nil && dockerfilePathVal != nil {
		return nil, fmt.Errorf("Cannot specify both dockerfile and dockerfile_contents keyword arguments")
	}
	if dockerfileContentsVal != nil {
		switch v := dockerfileContentsVal.(type) {
		case io.Blob:
			dockerfileContents = v.Text
		case starlark.String:
			dockerfileContents = v.GoString()
		default:
			return nil, fmt.Errorf("Argument (dockerfile_contents): must be string or blob.")
		}
	} else if dockerfilePathVal != nil {
		dockerfilePath, err = value.ValueToAbsPath(thread, dockerfilePathVal)
		if err != nil {
			return nil, err
		}

		bs, err := io.ReadFile(thread, dockerfilePath)
		if err != nil {
			return nil, errors.Wrap(err, "error reading dockerfile")
		}
		dockerfileContents = string(bs)
	} else {
		bs, err := io.ReadFile(thread, dockerfilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading dockerfile")
		}
		dockerfileContents = string(bs)
	}

	if cacheVal != nil {
		s.logger.Warnf("%s", cacheObsoleteWarning)
	}

	liveUpdate, err := s.liveUpdateFromSteps(thread, liveUpdateVal)
	if err != nil {
		return nil, errors.Wrap(err, "live_update")
	}

	ignores, err := parseValuesToStrings(ignoreVal, "ignore")
	if err != nil {
		return nil, err
	}

	onlys, err := s.parseOnly(onlyVal)
	if err != nil {
		return nil, err
	}

	entrypointCmd, err := value.ValueToUnixCmd(entrypoint)
	if err != nil {
		return nil, err
	}

	var containerArgs model.OverrideArgs
	if containerArgsVal != nil {
		args, err := value.SequenceToStringSlice(containerArgsVal)
		if err != nil {
			return nil, fmt.Errorf("Argument 'container_args': %v", err)
		}
		containerArgs = model.OverrideArgs{ShouldOverride: true, Args: args}
	}

	for _, extraTag := range extraTags.Values {
		_, err := container.ParseNamed(extraTag)
		if err != nil {
			return nil, fmt.Errorf("Argument extra_tag=%q not a valid image reference: %v", extraTag, err)
		}
	}

	r := &dockerImage{
		workDir:          starkit.CurrentExecPath(thread),
		dbDockerfilePath: dockerfilePath,
		dbDockerfile:     dockerfile.Dockerfile(dockerfileContents),
		dbBuildPath:      context,
		configurationRef: container.NewRefSelector(ref),
		dbBuildArgs:      sba,
		liveUpdate:       liveUpdate,
		matchInEnvVars:   matchInEnvVars,
		sshSpecs:         ssh.Values,
		secretSpecs:      secret.Values,
		ignores:          ignores,
		onlys:            onlys,
		entrypoint:       entrypointCmd,
		containerArgs:    containerArgs,
		targetStage:      targetStage,
		network:          network.Value,
		extraTags:        extraTags.Values,
		cacheFrom:        cacheFrom.Values,
		pullParent:       pullParent,
	}
	err = s.buildIndex.addImage(r)
	if err != nil {
		return nil, err
	}

	// NOTE(maia): docker_build returned a fast build that users can optionally
	// populate; now it just errors
	fb := &fastBuild{}
	return fb, nil
}

func (s *tiltfileState) parseOnly(val starlark.Value) ([]string, error) {
	paths, err := parseValuesToStrings(val, "only")
	if err != nil {
		return nil, err
	}

	for _, p := range paths {
		// We want to forbid file globs due to these issues:
		// https://github.com/tilt-dev/tilt/issues/1982
		// https://github.com/moby/moby/issues/30018
		if strings.Contains(p, "*") {
			return nil, fmt.Errorf("'only' does not support '*' file globs. Must be a real path: %s", p)
		}
	}
	return paths, nil
}

func (s *tiltfileState) customBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var dockerRef string
	var commandVal, commandBatVal starlark.Value
	var deps *starlark.List
	var tag string
	var disablePush bool
	var liveUpdateVal, ignoreVal starlark.Value
	var matchInEnvVars bool
	var entrypoint starlark.Value
	var containerArgsVal starlark.Sequence
	var skipsLocalDocker bool

	err := s.unpackArgs(fn.Name(), args, kwargs,
		"ref", &dockerRef,
		"command", &commandVal,
		"deps", &deps,
		"tag?", &tag,
		"disable_push?", &disablePush,
		"skips_local_docker?", &skipsLocalDocker,
		"live_update?", &liveUpdateVal,
		"match_in_env_vars?", &matchInEnvVars,
		"ignore?", &ignoreVal,
		"entrypoint?", &entrypoint,
		"container_args?", &containerArgsVal,
		"command_bat_val", &commandBatVal,
	)
	if err != nil {
		return nil, err
	}

	ref, err := container.ParseNamed(dockerRef)
	if err != nil {
		return nil, fmt.Errorf("Argument 1 (ref): can't parse %q: %v", dockerRef, err)
	}

	if deps == nil || deps.Len() == 0 {
		return nil, fmt.Errorf("Argument 3 (deps) can't be empty")
	}

	var localDeps []string
	iter := deps.Iterate()
	defer iter.Done()
	var v starlark.Value
	for iter.Next(&v) {
		p, err := value.ValueToAbsPath(thread, v)
		if err != nil {
			return nil, fmt.Errorf("Argument 3 (deps): %v", err)
		}
		localDeps = append(localDeps, p)
	}

	liveUpdate, err := s.liveUpdateFromSteps(thread, liveUpdateVal)
	if err != nil {
		return nil, errors.Wrap(err, "live_update")
	}

	ignores, err := parseValuesToStrings(ignoreVal, "ignore")
	if err != nil {
		return nil, err
	}

	entrypointCmd, err := value.ValueToUnixCmd(entrypoint)
	if err != nil {
		return nil, err
	}

	var containerArgs model.OverrideArgs
	if containerArgsVal != nil {
		args, err := value.SequenceToStringSlice(containerArgsVal)
		if err != nil {
			return nil, fmt.Errorf("Argument 'container_args': %v", err)
		}
		containerArgs = model.OverrideArgs{ShouldOverride: true, Args: args}
	}

	command, err := value.ValueGroupToCmdHelper(commandVal, commandBatVal)
	if err != nil {
		return nil, fmt.Errorf("Argument 2 (command): %v", err)
	} else if command.Empty() {
		return nil, fmt.Errorf("Argument 2 (command) can't be empty")
	}

	img := &dockerImage{
		workDir:          starkit.AbsWorkingDir(thread),
		configurationRef: container.NewRefSelector(ref),
		customCommand:    command,
		customDeps:       localDeps,
		customTag:        tag,
		disablePush:      disablePush,
		skipsLocalDocker: skipsLocalDocker,
		liveUpdate:       liveUpdate,
		matchInEnvVars:   matchInEnvVars,
		ignores:          ignores,
		entrypoint:       entrypointCmd,
		containerArgs:    containerArgs,
	}

	err = s.buildIndex.addImage(img)
	if err != nil {
		return nil, err
	}

	return &customBuild{s: s, img: img}, nil
}

type customBuild struct {
	s   *tiltfileState
	img *dockerImage
}

var _ starlark.Value = &customBuild{}

func (b *customBuild) String() string {
	return fmt.Sprintf("custom_build(%q)", b.img.configurationRef.String())
}

func (b *customBuild) Type() string {
	return "custom_build"
}

func (b *customBuild) Freeze() {}

func (b *customBuild) Truth() starlark.Bool {
	return true
}

func (b *customBuild) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: custom_build")
}

func (b *customBuild) Attr(name string) (starlark.Value, error) {
	switch name {
	case "add_fast_build":
		return nil, fastBuildDeletedErr
	default:
		return nil, nil
	}
}

func (b *customBuild) AttrNames() []string {
	return []string{}
}

func parseValuesToStrings(value starlark.Value, param string) ([]string, error) {

	tempIgnores := starlarkValueOrSequenceToSlice(value)
	var ignores []string
	for _, v := range tempIgnores {
		switch val := v.(type) {
		case starlark.String: // for singular string
			goString := val.GoString()
			if strings.Contains(goString, "\n") {
				return nil, fmt.Errorf(param+" cannot contain newlines; found "+param+": %q", goString)
			}
			ignores = append(ignores, val.GoString())
		default:
			return nil, fmt.Errorf(param+" must be a string or a sequence of strings; found a %T", val)
		}
	}
	return ignores, nil

}
func (s *tiltfileState) fastBuild(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return nil, fastBuildDeletedErr
}

// fastBuild exists just to error
type fastBuild struct {
}

var _ starlark.Value = &fastBuild{}

func (b *fastBuild) String() string {
	return "fast_build(%q)"
}

func (b *fastBuild) Type() string {
	return "fast_build"
}

func (b *fastBuild) Freeze() {}

func (b *fastBuild) Truth() starlark.Bool {
	return true
}

func (b *fastBuild) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: fast_build")
}

func (b *fastBuild) Attr(name string) (starlark.Value, error) {
	return nil, fastBuildDeletedErr
}

func (b *fastBuild) AttrNames() []string {
	return []string{}
}

func isGitRepoBase(path string) bool {
	return ospath.IsDir(filepath.Join(path, ".git"))
}

func reposForPaths(paths []string) []model.LocalGitRepo {
	var result []model.LocalGitRepo
	repoSet := map[string]bool{}

	for _, path := range paths {
		isRepoBase := isGitRepoBase(path)
		if !isRepoBase || repoSet[path] {
			continue
		}

		repoSet[path] = true
		result = append(result, model.LocalGitRepo{
			LocalPath: path,
		})
	}

	return result
}

func (s *tiltfileState) reposForImage(image *dockerImage) []model.LocalGitRepo {
	var paths []string
	paths = append(paths,
		image.dbDockerfilePath,
		image.dbBuildPath,
		image.workDir)
	paths = append(paths, image.customDeps...)

	return reposForPaths(paths)
}

func (s *tiltfileState) defaultRegistry(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !s.defaultReg.Empty() {
		return starlark.None, errors.New("default registry already defined")
	}

	var host, hostFromCluster string
	if err := s.unpackArgs(fn.Name(), args, kwargs,
		"host", &host,
		"host_from_cluster?", &hostFromCluster); err != nil {
		return nil, err
	}

	reg, err := container.NewRegistryWithHostFromCluster(host, hostFromCluster)
	if err != nil {
		return starlark.None, errors.Wrapf(err, "validating defaultRegistry")
	}

	s.defaultReg = reg

	return starlark.None, nil
}

func (s *tiltfileState) dockerignoresFromPathsAndContextFilters(paths []string, ignores []string, onlys []string, dbDockerfilePath string) []model.Dockerignore {
	var result []model.Dockerignore
	dupeSet := map[string]bool{}
	ignoreContents := ignoresToDockerignoreContents(ignores)
	onlyContents := onlysToDockerignoreContents(onlys)

	for _, path := range paths {
		if path == "" || dupeSet[path] {
			continue
		}
		dupeSet[path] = true

		if !ospath.IsDir(path) {
			continue
		}

		if ignoreContents != "" {
			result = append(result, model.Dockerignore{
				LocalPath: path,
				Contents:  ignoreContents,
			})
		}

		if onlyContents != "" {
			result = append(result, model.Dockerignore{
				LocalPath: path,
				Contents:  onlyContents,
			})
		}

		diFile := filepath.Join(path, ".dockerignore")
		customDiFile := dbDockerfilePath + ".dockerignore"
		_, err := os.Stat(customDiFile)
		if !os.IsNotExist(err) {
			diFile = customDiFile
		}

		s.postExecReadFiles = sliceutils.AppendWithoutDupes(s.postExecReadFiles, diFile)

		contents, err := ioutil.ReadFile(diFile)
		if err != nil {
			continue
		}

		result = append(result, model.Dockerignore{
			LocalPath: path,
			Contents:  string(contents),
		})
	}

	return result
}

func ignoresToDockerignoreContents(ignores []string) string {
	var output strings.Builder

	for _, ignore := range ignores {
		output.WriteString(ignore)
		output.WriteString("\n")
	}

	return output.String()
}

func onlysToDockerignoreContents(onlys []string) string {
	if len(onlys) == 0 {
		return ""
	}
	var output strings.Builder
	output.WriteString("**\n")

	for _, ignore := range onlys {
		output.WriteString("!")
		output.WriteString(ignore)
		output.WriteString("\n")
	}

	return output.String()
}

func (s *tiltfileState) dockerignoresForImage(image *dockerImage) []model.Dockerignore {
	var paths []string
	switch image.Type() {
	case DockerBuild:
		paths = append(paths, image.dbBuildPath)
	case CustomBuild:
		paths = append(paths, image.customDeps...)
	}
	return s.dockerignoresFromPathsAndContextFilters(paths, image.ignores, image.onlys, image.dbDockerfilePath)
}
