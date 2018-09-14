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
	Name    string
	Paths   []string
	k8sData k8sData
}

type k8sData struct {
	LoadBalancers []k8s.LoadBalancer
	Group         string
	Kinds         []string
	Version       string
}

// NewSummary returns summary state
func NewSummary() *Summary {
	return &Summary{
		Services: []*Service{},
	}
}

// Gather collates data into Summary
func (s *Summary) Gather(services []model.Manifest) error {

	for _, svc := range services {
		// Assume that, in practice, there is only one mount
		paths := []string{}
		if len(svc.Mounts) > 0 {
			for _, p := range svc.Mounts[0].Repo.LocalPaths {
				paths = append(paths, p)
			}
		}

		svcSummary := &Service{
			Name:  string(svc.Name),
			Paths: paths,
		}

		entities, err := k8s.ParseYAMLFromString(svc.K8sYaml)
		if err != nil {
			return err
		}

		kubeData := k8sData{
			LoadBalancers: k8s.ToLoadBalancers(entities),
		}

		for _, e := range entities {
			kubeData.Group = e.Kind.Group
			kubeData.Version = e.Kind.Version
			kubeData.Kinds = append(kubeData.Kinds, e.Kind.Kind)
		}

		svcSummary.k8sData = kubeData
		s.Services = append(s.Services, svcSummary)
	}

	return nil
}

func (s *Summary) Output() string {
	ret := "\n──┤ Services Built … ├────────────────────────────────────────\n"

	for _, svc := range s.Services {
		ret += fmt.Sprintf("    SERVICE NAME: %s\n", svc.Name)
		for _, p := range svc.Paths {
			ret += fmt.Sprintf("    WATCHING: %s\n", p)
		}

		k := svc.k8sData

		ret += fmt.Sprintln("    KUBERNETES INFO")
		if len(k.Version) > 0 {
			ret += fmt.Sprintf("      • Version: %s\n", k.Version)
		}

		if len(k.Group) > 0 {
			ret += fmt.Sprintf("      • Group: %s\n", k.Group)
		}

		if len(k.LoadBalancers) > 0 {
			ret += fmt.Sprintf("    LOAD BALANCER:")
			for _, lb := range k.LoadBalancers {
				ret += fmt.Sprintf(" %s", lb.Name)

				if len(lb.Ports) > 0 {
					for _, p := range lb.Ports {
						ret += fmt.Sprintf(" | PORT: %d", p)
						ret += fmt.Sprintf(" | URL: http://localhost:%d", p)
					}
					ret += fmt.Sprintf("\n")
				}
			}
		}

		if len(k.Kinds) > 0 {
			ret += fmt.Sprintln("    OBJECTS:")
			for _, kk := range k.Kinds {
				ret += fmt.Sprintf("    • %s\n", kk)
			}
		}

		ret += fmt.Sprintf("\n")
	}

	return ret
}
