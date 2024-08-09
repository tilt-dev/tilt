// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.21

package command

import (
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/context/store"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
)

const (
	// DefaultContextName is the name reserved for the default context (config & env based)
	DefaultContextName = "default"

	// EnvOverrideContext is the name of the environment variable that can be
	// used to override the context to use. If set, it overrides the context
	// that's set in the CLI's configuration file, but takes no effect if the
	// "DOCKER_HOST" env-var is set (which takes precedence.
	EnvOverrideContext = "DOCKER_CONTEXT"
)

// DefaultContext contains the default context data for all endpoints
type DefaultContext struct {
	Meta store.Metadata
	TLS  store.ContextTLSData
}

// DefaultContextResolver is a function which resolves the default context base on the configuration and the env variables
type DefaultContextResolver func() (*DefaultContext, error)

// ContextStoreWithDefault implements the store.Store interface with a support for the default context
type ContextStoreWithDefault struct {
	store.Store
	Resolver DefaultContextResolver
}

// EndpointDefaultResolver is implemented by any EndpointMeta object
// which wants to be able to populate the store with whatever their default is.
type EndpointDefaultResolver interface {
	// ResolveDefault returns values suitable for storing in store.Metadata.Endpoints
	// and store.ContextTLSData.Endpoints.
	//
	// An error is only returned for something fatal, not simply
	// the lack of a default (e.g. because the config file which
	// would contain it is missing). If there is no default then
	// returns nil, nil, nil.
	//
	//nolint:dupword // ignore "Duplicate words (nil,) found"
	ResolveDefault() (any, *store.EndpointTLSData, error)
}

// ResolveDefaultContext creates a Metadata for the current CLI invocation parameters
func ResolveDefaultContext(opts *cliflags.ClientOptions, config store.Config) (*DefaultContext, error) {
	contextTLSData := store.ContextTLSData{
		Endpoints: make(map[string]store.EndpointTLSData),
	}
	contextMetadata := store.Metadata{
		Endpoints: make(map[string]any),
		Metadata: DockerContext{
			Description: "",
		},
		Name: DefaultContextName,
	}

	dockerEP, err := resolveDefaultDockerEndpoint(opts)
	if err != nil {
		return nil, err
	}
	contextMetadata.Endpoints[docker.DockerEndpoint] = dockerEP.EndpointMeta
	if dockerEP.TLSData != nil {
		contextTLSData.Endpoints[docker.DockerEndpoint] = *dockerEP.TLSData.ToStoreTLSData()
	}

	if err := config.ForeachEndpointType(func(n string, get store.TypeGetter) error {
		if n == docker.DockerEndpoint { // handled above
			return nil
		}
		ep := get()
		if i, ok := ep.(EndpointDefaultResolver); ok {
			meta, tls, err := i.ResolveDefault()
			if err != nil {
				return err
			}
			if meta == nil {
				return nil
			}
			contextMetadata.Endpoints[n] = meta
			if tls != nil {
				contextTLSData.Endpoints[n] = *tls
			}
		}
		// Nothing to be done
		return nil
	}); err != nil {
		return nil, err
	}

	return &DefaultContext{Meta: contextMetadata, TLS: contextTLSData}, nil
}

// List implements store.Store's List
func (s *ContextStoreWithDefault) List() ([]store.Metadata, error) {
	contextList, err := s.Store.List()
	if err != nil {
		return nil, err
	}
	defaultContext, err := s.Resolver()
	if err != nil {
		return nil, err
	}
	return append(contextList, defaultContext.Meta), nil
}

// CreateOrUpdate is not allowed for the default context and fails
func (s *ContextStoreWithDefault) CreateOrUpdate(meta store.Metadata) error {
	if meta.Name == DefaultContextName {
		return errdefs.InvalidParameter(errors.New("default context cannot be created nor updated"))
	}
	return s.Store.CreateOrUpdate(meta)
}

// Remove is not allowed for the default context and fails
func (s *ContextStoreWithDefault) Remove(name string) error {
	if name == DefaultContextName {
		return errdefs.InvalidParameter(errors.New("default context cannot be removed"))
	}
	return s.Store.Remove(name)
}

// GetMetadata implements store.Store's GetMetadata
func (s *ContextStoreWithDefault) GetMetadata(name string) (store.Metadata, error) {
	if name == DefaultContextName {
		defaultContext, err := s.Resolver()
		if err != nil {
			return store.Metadata{}, err
		}
		return defaultContext.Meta, nil
	}
	return s.Store.GetMetadata(name)
}

// ResetTLSMaterial is not implemented for default context and fails
func (s *ContextStoreWithDefault) ResetTLSMaterial(name string, data *store.ContextTLSData) error {
	if name == DefaultContextName {
		return errdefs.InvalidParameter(errors.New("default context cannot be edited"))
	}
	return s.Store.ResetTLSMaterial(name, data)
}

// ResetEndpointTLSMaterial is not implemented for default context and fails
func (s *ContextStoreWithDefault) ResetEndpointTLSMaterial(contextName string, endpointName string, data *store.EndpointTLSData) error {
	if contextName == DefaultContextName {
		return errdefs.InvalidParameter(errors.New("default context cannot be edited"))
	}
	return s.Store.ResetEndpointTLSMaterial(contextName, endpointName, data)
}

// ListTLSFiles implements store.Store's ListTLSFiles
func (s *ContextStoreWithDefault) ListTLSFiles(name string) (map[string]store.EndpointFiles, error) {
	if name == DefaultContextName {
		defaultContext, err := s.Resolver()
		if err != nil {
			return nil, err
		}
		tlsfiles := make(map[string]store.EndpointFiles)
		for epName, epTLSData := range defaultContext.TLS.Endpoints {
			var files store.EndpointFiles
			for filename := range epTLSData.Files {
				files = append(files, filename)
			}
			tlsfiles[epName] = files
		}
		return tlsfiles, nil
	}
	return s.Store.ListTLSFiles(name)
}

// GetTLSData implements store.Store's GetTLSData
func (s *ContextStoreWithDefault) GetTLSData(contextName, endpointName, fileName string) ([]byte, error) {
	if contextName == DefaultContextName {
		defaultContext, err := s.Resolver()
		if err != nil {
			return nil, err
		}
		if defaultContext.TLS.Endpoints[endpointName].Files[fileName] == nil {
			return nil, errdefs.NotFound(errors.Errorf("TLS data for %s/%s/%s does not exist", DefaultContextName, endpointName, fileName))
		}
		return defaultContext.TLS.Endpoints[endpointName].Files[fileName], nil
	}
	return s.Store.GetTLSData(contextName, endpointName, fileName)
}

// GetStorageInfo implements store.Store's GetStorageInfo
func (s *ContextStoreWithDefault) GetStorageInfo(contextName string) store.StorageInfo {
	if contextName == DefaultContextName {
		return store.StorageInfo{MetadataPath: "<IN MEMORY>", TLSPath: "<IN MEMORY>"}
	}
	return s.Store.GetStorageInfo(contextName)
}
