package tiltfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/localexec"
	tiltfile_io "github.com/tilt-dev/tilt/internal/tiltfile/io"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/value"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/kustomize"
)

const localLogPrefix = " → "

type execCommandOptions struct {
	// logOutput writes stdout and stderr to logs if true.
	logOutput bool
	// logCommand writes the command being executed to logs if true.
	logCommand bool
	// logCommandPrefix is a custom prefix before the command (default: "Running: ") used if logCommand is true.
	logCommandPrefix string
	// stdin, if non-nil, will be written to the command's stdin
	stdin *string
}

func (s *tiltfileState) local(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var commandValue, commandBatValue, commandDirValue starlark.Value
	var commandEnv value.StringStringMap
	var stdin value.Stringable
	quiet := false
	echoOff := false
	err := s.unpackArgs(fn.Name(), args, kwargs,
		"command", &commandValue,
		"quiet?", &quiet,
		"command_bat", &commandBatValue,
		"echo_off", &echoOff,
		"env", &commandEnv,
		"dir?", &commandDirValue,
		"stdin?", &stdin,
	)
	if err != nil {
		return nil, err
	}

	cmd, err := value.ValueGroupToCmdHelper(thread, commandValue, commandBatValue, commandDirValue, commandEnv)
	if err != nil {
		return nil, err
	}

	execOptions := execCommandOptions{
		logOutput:        !quiet,
		logCommand:       !echoOff,
		logCommandPrefix: "local:",
	}
	if stdin.IsSet {
		s := stdin.Value
		execOptions.stdin = &s
	}
	out, err := s.execLocalCmd(thread, cmd, execOptions)
	if err != nil {
		return nil, err
	}

	return tiltfile_io.NewBlob(out, fmt.Sprintf("local: %s", cmd)), nil
}

func (s *tiltfileState) execLocalCmd(t *starlark.Thread, cmd model.Cmd, options execCommandOptions) (string, error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	ctx, err := starkit.ContextFromThread(t)
	if err != nil {
		return "", err
	}

	if options.logCommand {
		prefix := options.logCommandPrefix
		if prefix == "" {
			prefix = "Running:"
		}
		s.logger.Infof("%s %s", prefix, cmd)
	}

	var runIO localexec.RunIO
	if options.logOutput {
		logOutput := logger.NewMutexWriter(logger.NewPrefixedLogger(localLogPrefix, s.logger).Writer(logger.InfoLvl))
		runIO.Stdout = io.MultiWriter(&stdoutBuf, logOutput)
		runIO.Stderr = io.MultiWriter(&stderrBuf, logOutput)
	} else {
		runIO.Stdout = &stdoutBuf
		runIO.Stderr = &stderrBuf
	}

	if options.stdin != nil {
		runIO.Stdin = strings.NewReader(*options.stdin)
	}

	// TODO(nick): Should this also inject any docker.Env overrides?
	exitCode, err := s.execer.Run(ctx, cmd, runIO)
	if err != nil || exitCode != 0 {
		var errMessage strings.Builder
		errMessage.WriteString(fmt.Sprintf("command %q failed.", cmd))
		if err != nil {
			errMessage.WriteString(fmt.Sprintf("\nerror: %v", err))
		} else {
			errMessage.WriteString(fmt.Sprintf("\nerror: exit status %d", exitCode))
		}

		if !options.logOutput {
			// if we already logged the output, don't include it in the error message to prevent it from
			// getting output 2x
			errMessage.WriteString(fmt.Sprintf("\nstdout: %q\nstderr: %q",
				stdoutBuf.String(), stderrBuf.String()))
		}

		return "", errors.New(errMessage.String())
	}

	// only show that there was no output if the command was echoed AND we wanted output logged
	// otherwise, it's confusing to get "[no output]" without context of _what_ didn't have output
	if options.logCommand && options.logOutput && stdoutBuf.Len() == 0 && stderrBuf.Len() == 0 {
		s.logger.Infof("%s[no output]", localLogPrefix)
	}

	return stdoutBuf.String(), nil
}

func (s *tiltfileState) kustomize(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	path, kustomizeBin := value.NewLocalPathUnpacker(thread), value.NewLocalPathUnpacker(thread)
	flags := value.StringList{}
	err := s.unpackArgs(fn.Name(), args, kwargs, "paths", &path, "kustomize_bin?", &kustomizeBin, "flags?", &flags)
	if err != nil {
		return nil, err
	}

	kustomizeArgs := []string{"kustomize", "build"}

	if kustomizeBin.Value != "" {
		kustomizeArgs[0] = kustomizeBin.Value
	}

	_, err = exec.LookPath(kustomizeArgs[0])
	if err != nil {
		if kustomizeBin.Value != "" {
			return nil, err
		}
		s.logger.Infof("Falling back to `kubectl kustomize` since `%s` was not found in PATH", kustomizeArgs[0])
		kustomizeArgs = []string{"kubectl", "kustomize"}
	}

	// NOTE(nick): There's a bug in kustomize where it doesn't properly
	// handle absolute paths. Convert to relative paths instead:
	// https://github.com/kubernetes-sigs/kustomize/issues/2789
	relKustomizePath, err := filepath.Rel(starkit.AbsWorkingDir(thread), path.Value)
	if err != nil {
		return nil, err
	}

	cmd := model.Cmd{Argv: append(append(kustomizeArgs, flags...), relKustomizePath), Dir: starkit.AbsWorkingDir(thread)}
	yaml, err := s.execLocalCmd(thread, cmd, execCommandOptions{
		logOutput:  false,
		logCommand: true,
	})
	if err != nil {
		return nil, err
	}
	deps, err := kustomize.Deps(path.Value)
	if err != nil {
		return nil, fmt.Errorf("resolving deps: %v", err)
	}
	for _, d := range deps {
		err := tiltfile_io.RecordReadPath(thread, tiltfile_io.WatchRecursive, d)
		if err != nil {
			return nil, err
		}
	}

	return tiltfile_io.NewBlob(yaml, fmt.Sprintf("kustomize: %s", path.Value)), nil
}

func (s *tiltfileState) helm(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	path := value.NewLocalPathUnpacker(thread)
	var name string
	var namespace string
	var valueFiles value.StringOrStringList
	var set value.StringOrStringList
	var kubeVersion string

	err := s.unpackArgs(fn.Name(), args, kwargs,
		"paths", &path,
		"name?", &name,
		"namespace?", &namespace,
		"values?", &valueFiles,
		"set?", &set,
		"kube_version?", &kubeVersion,
	)
	if err != nil {
		return nil, err
	}

	localPath := path.Value
	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Could not read Helm chart directory %q: does not exist", localPath)
		}
		return nil, fmt.Errorf("Could not read Helm chart directory %q: %v", localPath, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("helm() may only be called on directories with Chart.yaml: %q", localPath)
	}

	err = tiltfile_io.RecordReadPath(thread, tiltfile_io.WatchRecursive, localPath)
	if err != nil {
		return nil, err
	}

	deps, err := localSubchartDependenciesFromPath(localPath)
	if err != nil {
		return nil, err
	}
	for _, d := range deps {
		err = tiltfile_io.RecordReadPath(thread, tiltfile_io.WatchRecursive, starkit.AbsPath(thread, d))
		if err != nil {
			return nil, err
		}
	}

	version, err := getHelmVersion()
	if err != nil {
		return nil, err
	}

	var cmd []string

	if name == "" {
		// Use 'chart' as the release name, so that the release name is stable
		// across Tiltfile loads.
		// This looks like what helm does.
		// https://github.com/helm/helm/blob/e672a42efae30d45ddd642a26557dcdbf5a9f5f0/pkg/action/install.go#L562
		name = "chart"
	}

	if version == helmV3_1andAbove {
		cmd = []string{"helm", "template", name, localPath, "--include-crds"}
	} else if version == helmV3_0 {
		cmd = []string{"helm", "template", name, localPath}
	} else {
		cmd = []string{"helm", "template", localPath, "--name", name}
	}

	if namespace != "" {
		cmd = append(cmd, "--namespace", namespace)
	}

	if kubeVersion != "" {
		cmd = append(cmd, "--kube-version", kubeVersion)
	}

	for _, valueFile := range valueFiles.Values {
		cmd = append(cmd, "--values", valueFile)
		err := tiltfile_io.RecordReadPath(thread, tiltfile_io.WatchFileOnly, starkit.AbsPath(thread, valueFile))
		if err != nil {
			return nil, err
		}
	}
	for _, setArg := range set.Values {
		cmd = append(cmd, "--set", setArg)
	}

	stdout, err := s.execLocalCmd(thread, model.Cmd{Argv: cmd, Dir: starkit.AbsWorkingDir(thread)}, execCommandOptions{
		logOutput:  false,
		logCommand: true,
	})
	if err != nil {
		return nil, err
	}

	yaml := filterHelmTestYAML(stdout)

	if version == helmV3_0 {
		// Helm v3.0 has a bug where it doesn't include CRDs in the template output
		// https://github.com/tilt-dev/tilt/issues/3605
		crds, err := getHelmCRDs(localPath)
		if err != nil {
			return nil, err
		}
		yaml = strings.Join(append([]string{yaml}, crds...), "\n---\n")
	}

	if namespace != "" {
		// helm template --namespace doesn't inject the namespace, nor provide
		// YAML that defines the namespace, so we have to do both ourselves :\
		// https://github.com/helm/helm/issues/5465
		parsed, err := k8s.ParseYAMLFromString(yaml)
		if err != nil {
			return nil, err
		}

		for i, e := range parsed {
			parsed[i] = e.WithNamespace(e.NamespaceOrDefault(namespace))
		}

		yaml, err = k8s.SerializeSpecYAML(parsed)
		if err != nil {
			return nil, err
		}
	}

	return tiltfile_io.NewBlob(yaml, fmt.Sprintf("helm: %s", localPath)), nil
}

// NOTE(nick): This isn't perfect. For example, it doesn't handle chart deps
// properly. When possible, prefer Helm 3.1's --include-crds
func getHelmCRDs(path string) ([]string, error) {
	crdPath := filepath.Join(path, "crds")
	result := []string{}
	err := filepath.Walk(crdPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		isYAML := info != nil && info.Mode().IsRegular() && hasYAMLExtension(path)
		if !isYAML {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		result = append(result, string(contents))
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return result, nil
}

func hasYAMLExtension(fname string) bool {
	ext := filepath.Ext(fname)
	return strings.EqualFold(ext, ".yaml") || strings.EqualFold(ext, ".yml")
}
