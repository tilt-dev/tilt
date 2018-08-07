package k8s

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

func Apply(ctx context.Context, rawYAML string) error {
	// TODO(dmiller) validate that the string is YAML and give a good error
	dir, err := ioutil.TempDir("", "tilt")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tmpf := filepath.Join(dir, "tilt.yaml")
	contents := []byte(rawYAML)
	if err := ioutil.WriteFile(tmpf, contents, 0666); err != nil {
		return err
	}
	c := exec.CommandContext(ctx, "kubectl", "apply", "-f", tmpf)
	return c.Run()
}
