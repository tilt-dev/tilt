package synclet

import (
	"context"
	"errors"
	"github.com/windmilleng/tilt/internal/model"
)

const Port = 23551

type Synclet struct {}

func (s Synclet) GetContainerIdForPod(podId string) (string, error) {
	return "", errors.New("GetContainerIdForPod not implemented")
}

func (s Synclet) UpdateContainer(
	ctx context.Context,
	containerId string,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

	return errors.New("UpdateContainer not implemented")
}