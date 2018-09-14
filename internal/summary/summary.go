package summary

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// Summary contains data to be printed at the end of the build process
type Summary struct {
	Services []*Service
}

type Service struct {
	Name       string
	Path       string
	K8sObjects []k8sObject
	K8sLbs     []k8s.LoadBalancer
}

type k8sObject struct {
	Kind string
	Name string
}

// NewSummary returns summary state
func NewSummary() *Summary {
	return &Summary{
		Services: []*Service{},
	}
}

// Gather collects summary data
func (s *Summary) Gather(services []model.Manifest) error {

	for _, svc := range services {
		// Assume that, in practice, there is only one mount
		path := ""
		if len(svc.Mounts) > 0 {
			path = svc.Mounts[0].Repo.LocalPath
		}

		entities, err := k8s.ParseYAMLFromString(svc.K8sYaml)
		if err != nil {
			return err
		}

		svcSummary := &Service{
			Name:   string(svc.Name),
			Path:   path,
			K8sLbs: k8s.ToLoadBalancers(entities),
		}

		for _, e := range entities {
			svcSummary.K8sObjects = append(svcSummary.K8sObjects, k8sObject{
				Name: e.Name(),
				Kind: e.Kind.Kind,
			})
		}

		s.Services = append(s.Services, svcSummary)
	}

	return nil
}

// Output prints the summary
func (s *Summary) Output() string {
	ret := ""
	for _, svc := range s.Services {
		indent := " "
		ret += fmt.Sprintf("%s%s: ", indent, svc.Name)

		// Relative Path
		if svc.Path != "" {
			wd, _ := os.Getwd()
			rel, err := filepath.Rel(wd, svc.Path)
			if err != nil {
				log.Fatalf("Failed to get relative path: %s", err)
			}
			ret += fmt.Sprintf("./%s ", rel)
		}

		// K8s — assume that the first name will work
		// TODO(han) - get the LoadBalancer kind (ie: "service") dynamically
		if len(svc.K8sLbs) > 0 {
			ret += fmt.Sprintf("→ `kubectl get svc %s` ", svc.K8sObjects[0].Name)
			if len(svc.K8sLbs[0].Ports) > 0 {
				for _, p := range svc.K8sLbs[0].Ports {
					ret += fmt.Sprintf("[http://localhost:%d] ", p)
				}
			}
		} else {
			ret += fmt.Sprintf("→ `kubectl get %s %s` ", strings.ToLower(svc.K8sObjects[0].Kind), svc.K8sObjects[0].Name)
		}

		// Space after each service, except the last
		if svc != s.Services[len(s.Services)-1] {
			ret += fmt.Sprintf("\n")
		}

	}
	return ret
}
