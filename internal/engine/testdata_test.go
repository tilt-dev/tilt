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

const BlorgBackendYAML = `
# Template should be populated using populate_config_template.py

apiVersion: v1
kind: Service
metadata:
  name: devel-nick-lb-blorg-be
  labels:
    app: blorg
    owner: nick
    environment: devel
    tier: backend
spec:
  type: LoadBalancer
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: blorg
    owner: nick
    environment: devel
    tier: backend
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: devel-nick-blorg-be
spec:
  selector:
    matchLabels:
      app: blorg
      owner: nick
      environment: devel
      tier: backend
  template:
    metadata:
      name: devel-nick-blorg-be
      labels:
        app: blorg
        owner: nick
        environment: devel
        tier: backend
    spec:
      containers:
      - name: backend
        imagePullPolicy: Always
        image: gcr.io/blorg-dev/blorg-backend:devel-nick
        command: [
          "/app/server",
          "--dbAddr", "hissing-cockroach-cockroachdb:26257"
        ]
        ports:
        - containerPort: 8080
`

const BlorgBackendDockerfile = `
FROM go:1.10
`

var BlorgBackendService = model.Service{
	Name:           "blorg-backend",
	DockerfileTag:  "gcr.io/blorg-dev/blorg-backend",
	K8sYaml:        BlorgBackendYAML,
	DockerfileText: BlorgBackendDockerfile,
	Mounts: []model.Mount{
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: "blorg-backend"},
			ContainerPath: "/go/src/github.com/windmilleng/blorg-backend",
		},
	},
	Steps: []model.Cmd{
		model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/blorg-backend"}},
	},
	Entrypoint: model.Cmd{Argv: []string{"/go/bin/blorg-backend"}},
}
