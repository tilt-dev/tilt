/*
   Copyright 2020 The Compose Specification Authors.

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

package types

// ServiceHook is a command to exec inside container by some lifecycle events
type ServiceHook struct {
	Command     ShellCommand      `yaml:"command,omitempty" json:"command"`
	User        string            `yaml:"user,omitempty" json:"user,omitempty"`
	Privileged  bool              `yaml:"privileged,omitempty" json:"privileged,omitempty"`
	WorkingDir  string            `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
	Environment MappingWithEquals `yaml:"environment,omitempty" json:"environment,omitempty"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}
