package output

import (
	"github.com/windmilleng/tilt/internal/model"
)

// Summary contains data to be printed at the end of the build process
type Summary struct {
	services []*service
}

type service struct {
	name string
	path string
}

// NewSummary returns summary state
func NewSummary() *Summary {
	return &Summary{
		services: []*service{},
	}
}

// Gather collates data into Summary
func (s *Summary) Gather(services []model.Service) {

	for _, svc := range services {
		s.services = append(s.services, &service{
			name: string(svc.Name),
			// Assume that, in practice, there is only one mount
			path: string(svc.Mounts[0].Repo.LocalPath),
		})
	}
}
