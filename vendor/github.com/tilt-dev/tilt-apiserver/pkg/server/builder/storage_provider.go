package builder

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/rest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	regsitryrest "k8s.io/apiserver/pkg/registry/rest"
)

// singletonProvider ensures different versions of the same resource share storage
type singletonProvider struct {
	sync.Once
	Provider rest.ResourceHandlerProvider
	storage  regsitryrest.Storage
	err      error
}

func (s *singletonProvider) Get(
	scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (regsitryrest.Storage, error) {
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
