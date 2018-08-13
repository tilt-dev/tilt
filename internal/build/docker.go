package build

import (
	"archive/tar"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/opencontainers/go-digest"
	"github.com/windmilleng/tilt/internal/tiltd"
)

// Tilt always pushes to the same tag. We never push to latest.
const pushTag = "wm-tilt"

type localDockerBuilder struct {
	dcli *client.Client
}

type Builder interface {
	BuildDocker(ctx context.Context, baseDockerfile string, mounts []tiltd.Mount, steps []tiltd.Cmd, entrypoint tiltd.Cmd) (digest.Digest, error)
	PushDocker(ctx context.Context, name string, dig digest.Digest) error
}

var _ Builder = &localDockerBuilder{}

func NewLocalDockerBuilder(cli *client.Client) *localDockerBuilder {
	return &localDockerBuilder{cli}
}

// NOTE(dmiller): not fully implemented yet
func (l *localDockerBuilder) BuildDocker(ctx context.Context, baseDockerfile string, mounts []tiltd.Mount, steps []tiltd.Cmd, entrypoint tiltd.Cmd) (digest.Digest, error) {
	baseDigest, err := l.buildBaseWithMounts(ctx, baseDockerfile, mounts)
	if err != nil {
		return "", fmt.Errorf("buildBaseWithMounts: %v", err)
	}

	newDigest, err := l.execStepsOnImage(ctx, baseDigest, steps)
	if err != nil {
		return "", fmt.Errorf("execStepsOnImage: %v", err)
	}

	newDigest, err = l.imageWithEntrypoint(ctx, newDigest, entrypoint)
	if err != nil {
		return "", fmt.Errorf("imageWithEntrypoint: %v", err)
	}

	return newDigest, nil
}

// Naively tag the digest and push it up to the docker registry specified in the name.
//
// TODO(nick) In the future, I would like us to be smarter about checking if the kubernetes cluster
// we're running in has access to the given registry. And if it doesn't, we should either emit an
// error, or push to a registry that kubernetes does have access to (e.g., a local registry).
func (l *localDockerBuilder) PushDocker(ctx context.Context, name string, dig digest.Digest) error {
	ref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return fmt.Errorf("PushDocker: %v", err)
	}

	if reference.Domain(ref) == "" {
		return fmt.Errorf("PushDocker: no domain in container name: %s", ref)
	}

	tag, err := reference.WithTag(ref, pushTag)
	if err != nil {
		return fmt.Errorf("PushDocker: %v", err)
	}

	err = l.dcli.ImageTag(ctx, dig.String(), tag.String())
	if err != nil {
		return fmt.Errorf("PushDocker#ImageTag: %v", err)
	}

	outputIO, err := l.dcli.ImagePush(
		ctx,
		tag.String(),
		types.ImagePushOptions{RegistryAuth: "{}"})
	if err != nil {
		return fmt.Errorf("PushDocker#ImagePush: %v", err)
	}
	return outputIO.Close()
}

func (l *localDockerBuilder) buildBaseWithMounts(ctx context.Context, baseDockerfile string, mounts []tiltd.Mount) (digest.Digest, error) {
	archive, err := tarContext(baseDockerfile, mounts)
	if err != nil {
		return "", err
	}
	// TODO(dmiller): remove this debugging code
	//tar2, err := tarContext(baseDockerfile, mounts)
	//if err != nil {
	//	return "", err
	//}
	//buf := new(bytes.Buffer)
	//buf.ReadFrom(tar2)
	//err = ioutil.WriteFile("/tmp/debug.tar", buf.Bytes(), os.FileMode(0777))
	//if err != nil {
	//	return "", err
	//}

	imageBuildResponse, err := l.dcli.ImageBuild(
		ctx,
		archive,
		types.ImageBuildOptions{
			Context:    archive,
			Dockerfile: "Dockerfile",
			Remove:     shouldRemoveImage(),
		})

	if err != nil {
		return "", err
	}

	defer imageBuildResponse.Body.Close()
	output := &strings.Builder{}
	_, err = io.Copy(output, imageBuildResponse.Body)
	if err != nil {
		return "", err
	}

	return getDigestFromOutput(output.String())
}

func (l *localDockerBuilder) execStepsOnImage(ctx context.Context, baseDigest digest.Digest, steps []tiltd.Cmd) (digest.Digest, error) {
	imageWithSteps := string(baseDigest)
	for _, s := range steps {
		cId, err := l.startContainer(ctx, imageWithSteps, &s)

		id, err := l.dcli.ContainerCommit(ctx, cId, types.ContainerCommitOptions{})
		if err != nil {
			return "", nil
		}
		imageWithSteps = id.ID
	}

	return digest.Digest(imageWithSteps), nil
}

// TODO(maia): can probs do this in a more efficient place -- e.g. `execStepsOnImage`
// already spins up + commits a container, maybe piggyback off that?
func (l *localDockerBuilder) imageWithEntrypoint(ctx context.Context, d digest.Digest, entrypoint tiltd.Cmd) (digest.Digest, error) {
	cId, err := l.startContainer(ctx, string(d), nil)

	opts := types.ContainerCommitOptions{
		Changes: []string{entrypoint.EntrypointStr()},
	}

	id, err := l.dcli.ContainerCommit(ctx, cId, opts)
	if err != nil {
		return "", nil
	}

	return digest.Digest(id.ID), nil
}

// startContainer starts a container from the given image ref, exec'ing a command if given.
// Returns the container id iff the container successfully starts up; else, error.
func (l *localDockerBuilder) startContainer(ctx context.Context, imgRef string, cmd *tiltd.Cmd) (cId string, err error) {
	config := &container.Config{Image: imgRef}
	if cmd != nil {
		config.Cmd = cmd.Argv
	}
	resp, err := l.dcli.ContainerCreate(ctx, config, nil, nil, "")
	if err != nil {
		return "", nil
	}
	containerID := resp.ID

	err = l.dcli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return "", err
	}

	statusCh, errCh := l.dcli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	}

	return containerID, nil
}

// tarContext amends the dockerfile with appropriate ADD statements,
// and returns that new dockerfile + necessary files in a tar
func tarContext(df string, mounts []tiltd.Mount) (*bytes.Reader, error) {
	buf := new(bytes.Buffer)
	err := tarToBuf(buf, df, mounts)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

func tarToBuf(buf *bytes.Buffer, df string, mounts []tiltd.Mount) error {
	tw := tar.NewWriter(buf)
	defer func() {
		err := tw.Close()
		if err != nil {
			log.Printf("Error closing tar writer: %s", err.Error())
		}
	}()

	// We'll tar all mounts so that their contents live inside their own dest
	// directories; since we generate the tar properly, can just dump everything
	// from the root into the container at /
	newdf := fmt.Sprintf("%s\nADD . /", df)

	tarHeader := &tar.Header{
		Name: "Dockerfile",
		Size: int64(len(newdf)),
	}
	err := tw.WriteHeader(tarHeader)
	if err != nil {
		return err
	}
	_, err = tw.Write([]byte(newdf))
	if err != nil {
		return err
	}

	for _, m := range mounts {
		err = tarPath(tw, m.Repo.LocalPath, m.ContainerPath)
		if err != nil {
			return err
		}
	}
	return nil
}

// tarPath writes the given source path into tarWriter at the given dest (recursively for directories).
// e.g. tarring my_dir --> dest d: d/file_a, d/file_b
func tarPath(tarWriter *tar.Writer, source, dest string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("%s: stat: %v", source, err)
	}

	sourceIsDir := sourceInfo.IsDir()
	if sourceIsDir {
		// Make sure we can trim this off filenames to get valid relative filepaths
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
	}

	dest = strings.TrimPrefix(dest, "/")

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking to %s: %v", path, err)
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("%s: making header: %v", path, err)
		}

		if sourceIsDir {
			// Name of file in tar should be relative to source directory...
			header.Name = strings.TrimPrefix(path, source)
		}

		if dest != "" {
			// ...and live inside `dest` (if given)
			header.Name = filepath.Join(dest, header.Name)
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("%s: writing header: %v", path, err)
		}

		if info.IsDir() {
			return nil
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("%s: open: %v", path, err)
			}
			defer file.Close()

			_, err = io.CopyN(tarWriter, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("%s: copying contents: %v", path, err)
			}
		}
		return nil
	})
}

func getDigestFromOutput(output string) (digest.Digest, error) {
	re := regexp.MustCompile(`{"aux":{"ID":"([[:alnum:]:]+)"}}`)
	res := re.FindStringSubmatch(output)
	if len(res) != 2 {
		return "", fmt.Errorf("digest not found in output: %s", output)
	}

	d, err := digest.Parse(res[1])
	if err != nil {
		return "", fmt.Errorf("getDigestFromOutput: %v", err)
	}

	return d, nil
}

func shouldRemoveImage() bool {
	if flag.Lookup("test.v") == nil {
		return false
	}
	return true
}
