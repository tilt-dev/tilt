package versions

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"

	registryclient "github.com/docker/cli/cli/registry/client"
	clitypes "github.com/docker/cli/types"
	"github.com/docker/distribution/reference"
	ver "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// defaultRuntimeMetadataDir is the location where the metadata file is stored
	defaultRuntimeMetadataDir = "/var/lib/docker-engine"
)

// GetEngineVersions reports the versions of the engine that are available
func GetEngineVersions(ctx context.Context, registryClient registryclient.RegistryClient, registryPrefix, imageName, versionString string) (clitypes.AvailableVersions, error) {

	if imageName == "" {
		var err error
		localMetadata, err := GetCurrentRuntimeMetadata("")
		if err != nil {
			return clitypes.AvailableVersions{}, err
		}
		imageName = localMetadata.EngineImage
	}
	imageRef, err := reference.ParseNormalizedNamed(path.Join(registryPrefix, imageName))
	if err != nil {
		return clitypes.AvailableVersions{}, err
	}

	tags, err := registryClient.GetTags(ctx, imageRef)
	if err != nil {
		return clitypes.AvailableVersions{}, err
	}

	return parseTags(tags, versionString)
}

func parseTags(tags []string, currentVersion string) (clitypes.AvailableVersions, error) {
	var ret clitypes.AvailableVersions
	currentVer, err := ver.NewVersion(currentVersion)
	if err != nil {
		return ret, errors.Wrapf(err, "failed to parse existing version %s", currentVersion)
	}
	downgrades := []clitypes.DockerVersion{}
	patches := []clitypes.DockerVersion{}
	upgrades := []clitypes.DockerVersion{}
	currentSegments := currentVer.Segments()
	for _, tag := range tags {
		tmp, err := ver.NewVersion(tag)
		if err != nil {
			logrus.Debugf("Unable to parse %s: %s", tag, err)
			continue
		}
		testVersion := clitypes.DockerVersion{Version: *tmp, Tag: tag}
		if testVersion.LessThan(currentVer) {
			downgrades = append(downgrades, testVersion)
			continue
		}
		testSegments := testVersion.Segments()
		// lib always provides min 3 segments
		if testSegments[0] == currentSegments[0] &&
			testSegments[1] == currentSegments[1] {
			patches = append(patches, testVersion)
		} else {
			upgrades = append(upgrades, testVersion)
		}
	}
	sort.Slice(downgrades, func(i, j int) bool {
		return downgrades[i].Version.LessThan(&downgrades[j].Version)
	})
	sort.Slice(patches, func(i, j int) bool {
		return patches[i].Version.LessThan(&patches[j].Version)
	})
	sort.Slice(upgrades, func(i, j int) bool {
		return upgrades[i].Version.LessThan(&upgrades[j].Version)
	})
	ret.Downgrades = downgrades
	ret.Patches = patches
	ret.Upgrades = upgrades
	return ret, nil
}

// GetCurrentRuntimeMetadata loads the current daemon runtime metadata information from the local host
func GetCurrentRuntimeMetadata(metadataDir string) (*clitypes.RuntimeMetadata, error) {
	if metadataDir == "" {
		metadataDir = defaultRuntimeMetadataDir
	}
	filename := filepath.Join(metadataDir, clitypes.RuntimeMetadataName+".json")

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var res clitypes.RuntimeMetadata
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, errors.Wrapf(err, "malformed runtime metadata file %s", filename)
	}
	return &res, nil
}

// WriteRuntimeMetadata stores the metadata on the local system
func WriteRuntimeMetadata(metadataDir string, metadata *clitypes.RuntimeMetadata) error {
	if metadataDir == "" {
		metadataDir = defaultRuntimeMetadataDir
	}
	filename := filepath.Join(metadataDir, clitypes.RuntimeMetadataName+".json")

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	os.Remove(filename)
	return ioutil.WriteFile(filename, data, 0644)
}
