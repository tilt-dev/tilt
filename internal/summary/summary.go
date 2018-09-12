package summary

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// Summary contains data to be printed at the end of the build process
type Summary struct {
	Services []*Service
}

type Service struct {
	Name     string
	Path     string
	K8sTypes []string
}

// NewSummary returns summary state
func NewSummary() *Summary {
	return &Summary{
		Services: []*Service{},
	}
}

// Gather collates data into Summary
func (s *Summary) Gather(services []model.Service) error {

	for _, svc := range services {
		// Assume that, in practice, there is only one mount
		path := ""
		if len(svc.Mounts) > 0 {
			path = svc.Mounts[0].Repo.LocalPath
		}

		svcSummary := &Service{
			Name: string(svc.Name),
			Path: path,
		}

		entities, err := k8s.ParseYAMLFromString(svc.K8sYaml)
		if err != nil {
			return err
		}

		for _, e := range entities {
			svcSummary.K8sTypes = append(svcSummary.K8sTypes, e.Kind.Kind)
		}

		s.Services = append(s.Services, svcSummary)
	}

	return nil
}

func (s *Summary) Output() string {
	ret := "\n──┤ Services Built … ├────────────────────────────────────────\n"

	for _, svc := range s.Services {
		ret += fmt.Sprintf("  • %s\n", svc.Name)
		ret += fmt.Sprintf("    %s\n", svc.Path)

		for _, t := range svc.K8sTypes {
			ret += fmt.Sprintf("    %s\n", t)
		}
	}

	return ret
}
