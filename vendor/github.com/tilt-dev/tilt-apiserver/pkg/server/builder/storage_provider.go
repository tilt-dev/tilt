package builder

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/rest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

// singletonProvider ensures different versions of the same resource share storage
type singletonProvider struct {
	sync.Once
	Provider rest.ResourceHandlerProvider
	storage  registryrest.Storage
	err      error
}

func (s *singletonProvider) Get(
	scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (registryrest.Storage, error) {
	s.Once.Do(func() {
		s.storage, s.err = s.Provider(scheme, optsGetter)
	})
	return s.storage, s.err
}

type errs struct {
	list []error
}

func (e errs) Error() string {
	msgs := []string{fmt.Sprintf("%d errors: ", len(e.list))}
	for i := range e.list {
		msgs = append(msgs, e.list[i].Error())
	}
	return strings.Join(msgs, "\n")
}

// Status resources only support: get, update, watch
type statusStorage struct {
	registryrest.Updater
	registryrest.Getter
}

type statusProvider struct {
	Provider rest.ResourceHandlerProvider
}

func (s *statusProvider) Get(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (registryrest.Storage, error) {
	storage, err := s.Provider(scheme, optsGetter)
	if err != nil {
		return nil, err
	}

	updater, ok := storage.(registryrest.Updater)
	if !ok {
		return nil, fmt.Errorf("status storage does not support update: %T", storage)
	}
	getter, ok := storage.(registryrest.Getter)
	if !ok {
		return nil, fmt.Errorf("status storage does not support get: %T", storage)
	}

	return &statusStorage{
		Updater: updater,
		Getter:  getter,
	}, nil
}
