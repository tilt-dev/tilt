package watch

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/builder/dockerignore"

	"github.com/tilt-dev/tilt/pkg/model"
)

const TiltignoreFileName = ".tiltignore"

// .tiltignore sits next to Tiltfile
func TiltignorePath(tiltfilePath string) string {
	return filepath.Join(filepath.Dir(tiltfilePath), TiltignoreFileName)
}

func ReadTiltignore(tiltignorePath string) (model.Dockerignore, error) {
	tiltignoreContents, err := ioutil.ReadFile(tiltignorePath)

	// missing tiltignore is fine, but a filesystem error is not
	if err != nil {
		if os.IsNotExist(err) {
			return model.Dockerignore{}, nil
		}
		return model.Dockerignore{}, err
	}

	patterns, err := dockerignore.ReadAll(bytes.NewBuffer(tiltignoreContents))
	if err != nil {
		return model.Dockerignore{}, fmt.Errorf("Parsing .tiltignore: %v", err)
	}

	return model.Dockerignore{
		LocalPath: filepath.Dir(tiltignorePath),
		Source:    tiltignorePath,
		Patterns:  patterns,
	}, nil
}
