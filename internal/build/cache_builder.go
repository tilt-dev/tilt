package build

import (
	"context"
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

// Cache directories are stored at the same image name, but with just the cachedir
// contents and at a particular tag.
const CacheTagPrefix = "tilt-cache-"

// Reads and writes images that contain cache directories.
//
// Used in the directory cache experimental feature, and designed to be deleted
// easily if that experiment doesn't work out.
//
// https://app.clubhouse.io/windmill/story/728/support-package-json-changes-gracefully
type CacheBuilder struct {
	dCli docker.Client
}

func NewCacheBuilder(dCli docker.Client) CacheBuilder {
	return CacheBuilder{
		dCli: dCli,
	}
}

func (b CacheBuilder) cacheRef(ref reference.Named, cachePaths []string) (reference.NamedTagged, error) {
	// Make an md5 hash of the cachePaths, so that when they change, the tag also changes.
	hashBuilder := md5.New()
	for _, p := range cachePaths {
		_, err := hashBuilder.Write([]byte(p))
		if err != nil {
			return nil, errors.Wrap(err, "CacheRef")
		}
	}

	hash := hashBuilder.Sum(nil)
	return reference.WithTag(ref, fmt.Sprintf("%s%x", CacheTagPrefix, hash))
}

func (b CacheBuilder) makeCacheDockerfile(baseDf dockerfile.Dockerfile, sourceRef reference.NamedTagged, cachePaths []string) dockerfile.Dockerfile {
	df := dockerfile.Dockerfile(fmt.Sprintf("FROM %s as tilt-source", sourceRef.String()))
	df = df.Join(string(baseDf))

	lines := []string{}
	for _, p := range cachePaths {
		lines = append(lines, fmt.Sprintf("COPY --from=tilt-source %s %s", p, p))
	}

	return df.Join(strings.Join(lines, "\n")).WithLabel(CacheImage, "1")
}

// Check if a cache exists for this ref name.
func (b CacheBuilder) FetchCache(ctx context.Context, ref reference.Named, cachePaths []string) (reference.NamedTagged, error) {
	// Nothing to do if there are no cache paths.
	if len(cachePaths) == 0 {
		return nil, nil
	}

	cacheRef, err := b.cacheRef(ref, cachePaths)
	if err != nil {
		return nil, err
	}

	_, _, err = b.dCli.ImageInspectWithRaw(ctx, cacheRef.String())
	if err == nil {
		// We found it! yay!
		return cacheRef, nil
	} else if !client.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "EnsureCacheExists")
	}

	return nil, nil
}

// Creates a cache image.
func (b CacheBuilder) CreateCacheFrom(ctx context.Context, baseDf dockerfile.Dockerfile, sourceRef reference.NamedTagged, cachePaths []string, buildArgs model.DockerBuildArgs) error {
	// Nothing to do if there are no cache paths
	if len(cachePaths) == 0 {
		return nil
	}

	cacheRef, err := b.cacheRef(sourceRef, cachePaths)
	if err != nil {
		return err
	}

	// Create a Dockerfile that copies directories from the sourceRef
	// and puts them in a standalone image.
	df := b.makeCacheDockerfile(baseDf, sourceRef, cachePaths)
	dockerCtx, err := tarDfOnly(ctx, df)
	if err != nil {
		return errors.Wrap(err, "CreateCacheFrom")
	}

	options := Options(dockerCtx, buildArgs)
	options.Tags = []string{cacheRef.String()}

	// TODO(nick): I'm not sure if we should print this, or if it should
	// be something that happens in the background without any user-visible output.
	writer := logger.Get(ctx).Writer(logger.DebugLvl)
	logger.Get(ctx).Debugf("Copying cache directories (%s)", sourceRef.String())
	res, err := b.dCli.ImageBuild(ctx, dockerCtx, options)
	if err != nil {
		return errors.Wrap(err, "ImageBuild")
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			logger.Get(ctx).Infof("unable to close imageBuildResponse: %s", err)
		}
	}()
	_, err = readDockerOutput(ctx, res.Body, writer)
	if err != nil {
		return errors.Wrap(err, "readDockerOutput")
	}
	return nil
}
