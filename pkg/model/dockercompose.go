package model

// A DockerComposeUpSpec describes how to apply
// DockerCompose service.
//
// We expect to become an API server object.
type DockerComposeUpSpec struct {
	// The name of the service to create.
	Service string

	// A specification of the project we're
	Project DockerComposeProject
}

type DockerComposeProject struct {
	// Configuration files to load.
	//
	// If both ConfigPaths and ProjectPath/YAML are specified,
	// the YAML is the source of truth, and the ConfigPaths
	// are used to print diagnostic information.
	ConfigPaths []string

	// The base path of the docker-compose project.
	//
	// Expressed in docker-compose as --project-directory.
	//
	// When used on the command-line, the Docker Compose spec mandates that this
	// must be the directory of the first yaml file.  All additional yaml files are
	// evaluated relative to this project path.
	ProjectPath string

	// The docker-compose config YAML.
	//
	// Usually contains multiple services.
	//
	// If you have multiple docker-compose.yaml files, you can combine them into a
	// single YAML with `docker-compose -f file1.yaml -f file2.yaml config`.
	YAML string
}

func IsEmptyDockerComposeProject(p DockerComposeProject) bool {
	return len(p.ConfigPaths) == 0 && p.YAML == ""
}
