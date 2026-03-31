package confutil

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"

	buildkitdconfig "github.com/moby/buildkit/cmd/buildkitd/config"
	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"
)

const (
	// DefaultBuildKitStateDir and DefaultBuildKitConfigDir are the location
	// where buildkitd inside the container stores its state. Some drivers
	// create a Linux container, so this should match the location for Linux,
	// as defined in: https://github.com/moby/buildkit/blob/v0.9.0/util/appdefaults/appdefaults_unix.go#L11-L15
	DefaultBuildKitStateDir  = "/var/lib/buildkit"
	DefaultBuildKitConfigDir = "/etc/buildkit"
)

var reInvalidCertsDir = regexp.MustCompile(`[^a-zA-Z0-9.-]+`)

// LoadConfigFiles creates a temp directory with BuildKit config and
// registry certificates ready to be copied to a container.
func LoadConfigFiles(bkconfig string) (map[string][]byte, error) {
	if _, err := os.Stat(bkconfig); errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "buildkit configuration file not found: %s", bkconfig)
	} else if err != nil {
		return nil, errors.Wrapf(err, "invalid buildkit configuration file: %s", bkconfig)
	}
	dt, err := readFile(bkconfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read buildkit configuration file: %s", bkconfig)
	}

	cfg, err := buildkitdconfig.LoadFile(bkconfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load buildkit configuration file: %s", bkconfig)
	}

	m := make(map[string][]byte)
	// unmarshal the config file to a map because marshalling the struct back can cause errors with empty fields on buildkit side
	var conf map[string]any
	if err := toml.Unmarshal(dt, &conf); err != nil {
		return nil, errors.Wrapf(err, "failed to parse buildkit configuration file: %s", bkconfig)
	}
	if conf == nil {
		conf = map[string]any{}
	}
	registry, hadRegistry := conf["registry"].(map[string]any)
	if registry == nil {
		registry = map[string]any{}
	}

	// Iterate through registry config to copy certs and update
	// BuildKit config with the underlying certs' path in the container.
	//
	// The following BuildKit config:
	//
	// [registry."myregistry.io"]
	//   ca=["/etc/config/myca.pem"]
	//   [[registry."myregistry.io".keypair]]
	//     key="/etc/config/key.pem"
	//     cert="/etc/config/cert.pem"
	//
	// will be translated in the container as:
	//
	// [registry."myregistry.io"]
	//   ca=["/etc/buildkit/certs/myregistry.io/myca.pem"]
	//   [[registry."myregistry.io".keypair]]
	//     key="/etc/buildkit/certs/myregistry.io/key.pem"
	//     cert="/etc/buildkit/certs/myregistry.io/cert.pem"
	if cfg.Registries != nil {
		for regName, regConf := range cfg.Registries {
			regOut, ok := registry[regName].(map[string]any)
			if !ok {
				return nil, errors.Errorf("invalid registry config for %q", regName)
			}

			pfx := path.Join("certs", reInvalidCertsDir.ReplaceAllString(regName, "_"))
			if regCAs := regConf.RootCAs; len(regCAs) > 0 {
				cas := make([]string, 0, len(regCAs))
				for _, ca := range regCAs {
					fp := path.Join(pfx, filepath.Base(ca))
					dst := path.Join(DefaultBuildKitConfigDir, fp)
					cas = append(cas, dst)

					dt, err := readFile(ca)
					if err != nil {
						return nil, errors.Wrapf(err, "failed to read CA file: %s", ca)
					}
					m[fp] = dt
				}
				regOut["ca"] = cas
			}
			if regKeyPairs := regConf.KeyPairs; len(regKeyPairs) > 0 {
				keypairs := make([]map[string]any, 0, len(regKeyPairs))
				for _, kp := range regKeyPairs {
					kpv := map[string]any{}
					key := kp.Key
					if len(key) > 0 {
						fp := path.Join(pfx, filepath.Base(key))
						dst := path.Join(DefaultBuildKitConfigDir, fp)
						kpv["key"] = dst
						dt, err := readFile(key)
						if err != nil {
							return nil, errors.Wrapf(err, "failed to read key file: %s", key)
						}
						m[fp] = dt
					}
					cert := kp.Certificate
					if len(cert) > 0 {
						fp := path.Join(pfx, filepath.Base(cert))
						dst := path.Join(DefaultBuildKitConfigDir, fp)
						kpv["cert"] = dst
						dt, err := readFile(cert)
						if err != nil {
							return nil, errors.Wrapf(err, "failed to read cert file: %s", cert)
						}
						m[fp] = dt
					}
					keypairs = append(keypairs, kpv)
				}
				regOut["keypair"] = keypairs
			}
			registry[regName] = regOut
		}
	}

	if hadRegistry || len(registry) > 0 {
		conf["registry"] = registry
	}
	out, err := toml.Marshal(conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal buildkit configuration file")
	}
	m["buildkitd.toml"] = out

	return m, nil
}

func readFile(fp string) ([]byte, error) {
	sf, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer sf.Close()
	return io.ReadAll(io.LimitReader(sf, 1024*1024))
}
