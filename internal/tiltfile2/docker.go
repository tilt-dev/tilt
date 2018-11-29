package tiltfile2

import ()

type dockerImage struct {
	baseDockerfilePath localPath
	baseDockerfile     dockerfile.Dockerfile
	ref                reference.Named
	mounts             []mount
	steps              []model.Step
	entrypoint         string
	tiltFilename       string
	cachePaths         []string

	staticDockerfilePath localPath
	staticDockerfile     dockerfile.Dockerfile
	staticBuildPath      localPath
	staticBuildArgs      model.DockerBuildArgs
}

func (t *Tiltfile) readDockerfile(thread *skylark.Thread, path string) (dockerfile.Dockerfile, error) {
	err := t.recordReadFile(thread, path)
	if err != nil {
		return "", err
	}

	dfBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to open dockerfile '%v': %v", path, err)
	}

	return dockerfile.Dockerfile(dfBytes), nil
}
