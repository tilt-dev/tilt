package tiltfile2

import ()

type k8sResource struct {
	name     string
	k8sYaml  string
	imageRef string

	// FIXME(dbentley): port forwards
}
