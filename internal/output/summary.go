package output

import (
	"github.com/windmilleng/tilt/internal/model"
)

// Summary contains data to be printed at the end of the build process
type summary struct {
	services []string
}

// NewSummary returns summary state
func NewSummary() *summary {
	return &summary{
		services: []string{},
	}
}

// Gather collates data into Summary
func (s *summary) Gather(services []model.Service) {
	for _, svc := range services {
		s.services = append(s.services, string(svc.Name))
	}
}
