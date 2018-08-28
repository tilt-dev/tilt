/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package oci

import (
	"context"

	"github.com/containerd/containerd/containers"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Spec is a type alias to the OCI runtime spec to allow third part SpecOpts
// to be created without the "issues" with go vendoring and package imports
type Spec = specs.Spec

// GenerateSpec will generate a default spec from the provided image
// for use as a containerd container
func GenerateSpec(ctx context.Context, client Client, c *containers.Container, opts ...SpecOpts) (*Spec, error) {
	s, err := createDefaultSpec(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	return s, ApplyOpts(ctx, client, c, s, opts...)
}

// ApplyOpts applys the options to the given spec, injecting data from the
// context, client and container instance.
func ApplyOpts(ctx context.Context, client Client, c *containers.Container, s *Spec, opts ...SpecOpts) error {
	for _, o := range opts {
		if err := o(ctx, client, c, s); err != nil {
			return err
		}
	}

	return nil
}

func createDefaultSpec(ctx context.Context, id string) (*Spec, error) {
	var s Spec
	return &s, populateDefaultSpec(ctx, &s, id)
}
