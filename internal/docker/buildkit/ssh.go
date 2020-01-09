/**
Code for parsing Buildkit arguments adapted from Docker CLI

Adapted from
https://github.com/docker/cli/blob/28ac2f82c690ac8f37c2980f2787572815439a50/cli/command/image/build_buildkit.go


   Copyright 2013-2017 Docker, Inc.

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

package buildkit

import (
	"strings"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
)

func ParseSSHSpecs(sl []string) (session.Attachable, error) {
	configs := make([]sshprovider.AgentConfig, 0, len(sl))
	for _, v := range sl {
		c := parseSSH(v)
		configs = append(configs, *c)
	}
	return sshprovider.NewSSHAgentProvider(configs)
}

func parseSSH(value string) *sshprovider.AgentConfig {
	parts := strings.SplitN(value, "=", 2)
	cfg := sshprovider.AgentConfig{
		ID: parts[0],
	}
	if len(parts) > 1 {
		cfg.Paths = strings.Split(parts[1], ",")
	}
	return &cfg
}
