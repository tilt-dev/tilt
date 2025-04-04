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

type DevelopConfig struct {
	Watch []Trigger `yaml:"watch,omitempty" json:"watch,omitempty"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

type WatchAction string

const (
	WatchActionSync        WatchAction = "sync"
	WatchActionRebuild     WatchAction = "rebuild"
	WatchActionRestart     WatchAction = "restart"
	WatchActionSyncRestart WatchAction = "sync+restart"
	WatchActionSyncExec    WatchAction = "sync+exec"
)

type Trigger struct {
	Path       string      `yaml:"path" json:"path"`
	Action     WatchAction `yaml:"action" json:"action"`
	Target     string      `yaml:"target,omitempty" json:"target,omitempty"`
	Exec       ServiceHook `yaml:"exec,omitempty" json:"exec,omitempty"`
	Include    []string    `yaml:"include,omitempty" json:"include,omitempty"`
	Ignore     []string    `yaml:"ignore,omitempty" json:"ignore,omitempty"`
	Extensions Extensions  `yaml:"#extensions,inline,omitempty" json:"-"`
}
