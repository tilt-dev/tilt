package build

import "github.com/windmilleng/tilt/internal/model"

// FileOp represents an operation we want to perform on the container -- either
// a. copying the file at LocalAbsPath --> <DestMount>/<DestPath>, or
// b. (if no file at LocalAbsPath exists) rm <DestMount>/<DestPath>
type FileOp struct {
	LocalAbsPath string // absolute path
	DestMount    string // folder to mount in on container (corresponds to ContainerPath on a model.Mount)
	DestPath     string // relative to DestMount
}

// FilesToOps converts a list of absolute local filepaths into FileOps (i.e.
// associates local filepaths with their mounts and destination paths).
func FilesToOps(files []string, mounts []model.Mount) ([]FileOp, error) {
	return nil, nil
}
