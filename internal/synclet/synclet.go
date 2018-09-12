package synclet

import (
	"context"
	"errors"
	"fmt"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/model"
)

const Port = 23551

type Synclet struct {
	dcli build.DockerClient
}

func NewSynclet(dcli build.DockerClient) *Synclet {
	return &Synclet{dcli: dcli}
}

func (s Synclet) GetContainerIdForPod(podId string) (string, error) {
	return "", errors.New("GetContainerIdForPod not implemented")
}

func (s Synclet) UpdateContainer(
	ctx context.Context,
	containerId string,
	tarArchive []byte,
	filesToDelete []string,
	commands []model.Cmd) error {

	fmt.Println("container:", containerId)
	fmt.Println("bytes in tar:", len(tarArchive))
	fmt.Println("files to delete:", filesToDelete)
	fmt.Println("cmds:", commands)

	return errors.New("UpdateContainer not implemented")
}
