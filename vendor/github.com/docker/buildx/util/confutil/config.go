package confutil

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/cli/cli/command"
	"github.com/moby/sys/atomicwriter"
	fs "github.com/tonistiigi/fsutil/copy"
)

const defaultBuildKitConfigFile = "buildkitd.default.toml"

type Config struct {
	dir     string
	chowner *chowner
}

type chowner struct {
	uid int
	gid int
}

type ConfigOption func(*configOptions)

type configOptions struct {
	dir string
}

func WithDir(dir string) ConfigOption {
	return func(o *configOptions) {
		o.dir = dir
	}
}

func NewConfig(dockerCli command.Cli, opts ...ConfigOption) *Config {
	co := configOptions{}
	for _, opt := range opts {
		opt(&co)
	}

	configDir := co.dir
	if configDir == "" {
		configDir = os.Getenv("BUILDX_CONFIG")
		if configDir == "" {
			configDir = filepath.Join(filepath.Dir(dockerCli.ConfigFile().Filename), "buildx")
		}
	}

	return &Config{
		dir:     configDir,
		chowner: sudoer(configDir),
	}
}

// Dir will look for correct configuration store path;
// if `$BUILDX_CONFIG` is set - use it, otherwise use parent directory
// of Docker config file (i.e. `${DOCKER_CONFIG}/buildx`)
func (c *Config) Dir() string {
	return c.dir
}

// BuildKitConfigFile returns the default BuildKit configuration file path
func (c *Config) BuildKitConfigFile() (string, bool) {
	f := filepath.Join(c.dir, defaultBuildKitConfigFile)
	if _, err := os.Stat(f); err == nil {
		return f, true
	}
	return "", false
}

// MkdirAll creates a directory and all necessary parents within the config dir.
func (c *Config) MkdirAll(dir string, perm os.FileMode) error {
	var chown fs.Chowner
	if c.chowner != nil {
		chown = func(user *fs.User) (*fs.User, error) {
			return &fs.User{UID: c.chowner.uid, GID: c.chowner.gid}, nil
		}
	}
	d := filepath.Join(c.dir, dir)
	st, err := os.Stat(d)
	if err != nil {
		if os.IsNotExist(err) {
			_, err := fs.MkdirAll(d, perm, chown, nil)
			return err
		}
		return err
	}
	// if directory already exists, fix the owner if necessary
	if c.chowner == nil {
		return nil
	}
	currentOwner := fileOwner(st)
	if currentOwner != nil && (currentOwner.uid != c.chowner.uid || currentOwner.gid != c.chowner.gid) {
		return os.Chown(d, c.chowner.uid, c.chowner.gid)
	}
	return nil
}

// AtomicWriteFile writes data to a file within the config dir atomically
func (c *Config) AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	f := filepath.Join(c.dir, filename)
	if err := atomicwriter.WriteFile(f, data, perm); err != nil {
		return err
	}
	if c.chowner == nil {
		return nil
	}
	return os.Chown(f, c.chowner.uid, c.chowner.gid)
}

var nodeIdentifierMu sync.Mutex

func (c *Config) TryNodeIdentifier() (out string) {
	nodeIdentifierMu.Lock()
	defer nodeIdentifierMu.Unlock()
	sessionFilename := ".buildNodeID"
	sessionFilepath := filepath.Join(c.Dir(), sessionFilename)
	if _, err := os.Lstat(sessionFilepath); err != nil {
		if os.IsNotExist(err) { // create a new file with stored randomness
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				return out
			}
			if err := c.AtomicWriteFile(sessionFilename, []byte(hex.EncodeToString(b)), 0600); err != nil {
				return out
			}
		}
	}
	dt, err := os.ReadFile(sessionFilepath)
	if err == nil {
		return string(dt)
	}
	return
}
