package summary

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// Summary contains data to be printed at the end of the build process
type Summary struct {
	Services []*Service
}

type Service struct {
	Name    string
	Path    string
	k8sData k8sData
}

type k8sData struct {
	Name          string
	LoadBalancers []k8s.LoadBalancer
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

		kubeData := k8sData{
			LoadBalancers: k8s.ToLoadBalancers(entities),
		}

		for _, e := range entities {
			kubeData.Name = k8s.GetDeploymentName(e)
		}

		svcSummary.k8sData = kubeData
		s.Services = append(s.Services, svcSummary)
	}

	return nil
}

func (s *Summary) Output() string {
	ret := ""
	for _, svc := range s.Services {
		indent := " "
		ret += fmt.Sprintf("%s%s — ", indent, svc.Name)

		// Relative Path
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %s", err)
		}
		rel, err := filepath.Rel(wd, svc.Path)
		if err != nil {
			log.Fatalf("Failed to get relative path: %s", err)
		}
		ret += fmt.Sprintf("./%s ", rel)

		// K8s info
		k := svc.k8sData
		if len(k.Name) > 0 {
			ret += fmt.Sprintf("→ `kubectl get deploy %s` ", k.Name)
		}
		if len(k.LoadBalancers) > 0 {
			for _, lb := range k.LoadBalancers {
				ret += fmt.Sprintf("→ `kubectl get svc %s` ", lb.Name)
				if len(lb.Ports) > 0 {
					for _, p := range lb.Ports {
						ret += fmt.Sprintf("[http://localhost:%d]", p)
					}
				}
			}
		}
		ret += fmt.Sprintf("\n") // newline after each service
	}
	return ret
}
