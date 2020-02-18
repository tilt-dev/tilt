package tiltextension

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

const extensionDirName = "tilt_modules"
const extensionFileName = "Tiltfile"
const metadataFileName = "extensions.json"

type Store interface {
	// ModulePath is used to check if an extension exists before fetching it
	// Returns ErrNotExist if module doesn't exist
	ModulePath(ctx context.Context, moduleName string) (string, error)
	Write(ctx context.Context, contents ModuleContents) (string, error)
}

// TODO(dmiller): should this include an integrity hash?
type ModuleContents struct {
	Name              string
	TiltfileContents  string
	GitCommitHash     string
	ExtensionRegistry string
	TimeFetched       time.Time
}

type LocalStore struct {
	baseDir string
}

type Metadata struct {
	Name              string
	GitCommitHash     string
	ExtensionRegistry string
	TimeFetched       time.Time
}

type MetadataFile struct {
	Extensions []Metadata
}

func NewLocalStore(baseDir string) *LocalStore {
	return &LocalStore{
		baseDir: filepath.Join(baseDir, extensionDirName),
	}
}

func (s *LocalStore) ModulePath(ctx context.Context, moduleName string) (string, error) {
	tiltfilePath := filepath.Join(s.baseDir, moduleName, extensionFileName)

	_, err := os.Stat(tiltfilePath)
	if err != nil {
		return "", err
	}

	return tiltfilePath, nil
}

// TODO(dmiller): handle atomic writes to the metadata file and the modules?
// Right now if a write to the metadata file fails the module will still be written

// TODO(dmiller): should this error if we try to write an extension with the same name as
// one that already exists?
func (s *LocalStore) Write(ctx context.Context, contents ModuleContents) (string, error) {
	moduleDir := filepath.Join(s.baseDir, contents.Name)
	if err := os.MkdirAll(moduleDir, os.FileMode(0700)); err != nil {
		return "", errors.Wrapf(err, "couldn't create module directory %s at path %s", contents.Name, moduleDir)
	}

	tiltfilePath := filepath.Join(moduleDir, extensionFileName)
	if err := ioutil.WriteFile(tiltfilePath, []byte(contents.TiltfileContents), os.FileMode(0600)); err != nil {
		return "", errors.Wrapf(err, "couldn't store module %s at path %s", contents.Name, tiltfilePath)
	}

	metadata := Metadata{
		Name:              contents.Name,
		GitCommitHash:     contents.GitCommitHash,
		ExtensionRegistry: contents.ExtensionRegistry,
		TimeFetched:       contents.TimeFetched,
	}

	// read file if it exists, append extension, write out the file
	var metadataFile MetadataFile
	extensionMetadataFilePath := filepath.Join(s.baseDir, metadataFileName)
	b, err := ioutil.ReadFile(extensionMetadataFilePath)
	if os.IsNotExist(err) {
		metadataFile = MetadataFile{
			Extensions: []Metadata{metadata},
		}
	} else if err != nil {
		return "", errors.Wrapf(err, "unable to open extension metadata file at path %s", extensionMetadataFilePath)
	} else {
		err = json.Unmarshal(b, &metadataFile)
		if err != nil {
			return "", errors.Wrapf(err, "Unable to unmarshal metadata file at path %s", extensionMetadataFilePath)
		}
		metadataFile.Extensions = append(metadataFile.Extensions, metadata)
	}

	js, err := json.MarshalIndent(metadataFile, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "internal error: unable to marshal metadataFile as JSON")
	}

	err = ioutil.WriteFile(extensionMetadataFilePath, js, 0600)
	if err != nil {
		return "", errors.Wrapf(err, "unable to write extension metadata file at path %s", extensionMetadataFilePath)
	}

	return tiltfilePath, nil
}

var _ Store = (*LocalStore)(nil)
