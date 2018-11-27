package dockercompose

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Service struct {
	Name string
}

func ParseConfig(ctx context.Context, files []string) ([]Service, []string, error) {
	var args []string
	for _, f := range files {
		args = append(args, "-f", f)
	}
	args = append(args, "config")
	_, err := DcOutput(ctx, args...)
	if err != nil {
		return nil, files, err
	}

	args = append(args, "--services")
	servicesText, err := DcOutput(ctx, args...)
	if err != nil {
		return nil, files, err
	}

	serviceNames := strings.Split(string(servicesText), "\n")

	var services []Service

	for _, name := range serviceNames {
		if name == "" {
			continue
		}
		services = append(services, Service{Name: name})
	}

	return services, files, nil
}

func DcOutput(ctx context.Context, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, "docker-compose", args...).Output()
	if err != nil {
		errorMessage := fmt.Sprintf("command 'docker-compose %q' failed.\nerror: '%v'\nstdout: '%v'", args, err, string(output))
		if err, ok := err.(*exec.ExitError); ok {
			errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
		}
		err = errors.New(errorMessage)
	}
	return string(output), err
}

func NiceError(cmd *exec.Cmd, stdout []byte, err error) error {
	if err == nil {
		return nil
	}
	errorMessage := fmt.Sprintf("command '%q %q' failed.\nerror: '%v'\n", cmd.Path, cmd.Args, err)
	if len(stdout) > 0 {
		errorMessage += fmt.Sprintf("\nstdout: '%v'", string(stdout))
	}
	if err, ok := err.(*exec.ExitError); ok && len(err.Stderr) > 0 {
		errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
	}
	return errors.New(errorMessage)
}
