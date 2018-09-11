package summary

import (
	"github.com/windmilleng/tilt/internal/model"
)

// Summary contains data to be printed at the end of the build process
type Summary struct {
	Services []*Service
}

type Service struct {
	Name string
	Path string
}

// NewSummary returns summary state
func NewSummary() *Summary {
	return &Summary{
		Services: []*Service{},
	}
}

// Gather collates data into Summary
func (s *Summary) Gather(services []model.Service) {

	for _, svc := range services {
		s.Services = append(s.Services, &Service{
			Name: string(svc.Name),
			// Assume that, in practice, there is only one mount
			Path: string(svc.Mounts[0].Repo.LocalPath),
		})
	}
}
