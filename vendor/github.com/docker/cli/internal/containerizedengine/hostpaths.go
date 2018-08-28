package containerizedengine

import (
	"os"
	"path"
)

func (c baseClient) verifyDockerConfig(configFile string) error {

	// TODO - in the future consider leveraging containerd and a host runtime
	// to create the file.  For now, just create it locally since we have to be
	// local to talk to containerd

	configDir := path.Dir(configFile)
	err := os.MkdirAll(configDir, 0644)
	if err != nil {
		return err
	}

	fd, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer fd.Close()

	info, err := fd.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		_, err := fd.Write([]byte("{}"))
		return err
	}
	return nil
}
