package summary

import (
	"fmt"

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
		var path string
		// Assume that, in practice, there is only one mount
		if len(svc.Mounts) > 0 {
			path = svc.Mounts[0].Repo.LocalPath
		} else {
			path = ""
		}
		s.Services = append(s.Services, &Service{
			Name: string(svc.Name),
			Path: path,
		})
	}
}

func (s *Summary) Output() string {
	ret := "\n──┤ Services Built … ├────────────────────────────────────────\n"

	for _, svc := range s.Services {
		ret += fmt.Sprintf("  • %s\n", svc.Name)
		ret += fmt.Sprintf("    (%s)\n", svc.Path)
	}

	return ret
}
