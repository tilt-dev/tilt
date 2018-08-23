package engine

import "github.com/windmilleng/tilt/internal/model"

const SanchoYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sancho
  labels:
    app: sancho
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sancho
  template:
    metadata:
      labels:
        app: sancho
    spec:
      containers:
      - name: sancho
        image: gcr.io/some-project-162817/sancho
        env:
          - name: token
            valueFrom:
              secretKeyRef:
                name: slacktoken
                key: token
`

const SanchoDockerfile = `
FROM go:1.10
`

var SanchoService = model.Service{
	Name:           "sancho",
	DockerfileTag:  "gcr.io/some-project-162817/sancho",
	K8sYaml:        SanchoYAML,
	DockerfileText: SanchoDockerfile,
	Mounts: []model.Mount{
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: "sancho"},
			ContainerPath: "/go/src/github.com/windmilleng/sancho",
		},
	},
	Steps: []model.Cmd{
		model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}},
	},
	Entrypoint: model.Cmd{Argv: []string{"/go/bin/sancho"}},
}
