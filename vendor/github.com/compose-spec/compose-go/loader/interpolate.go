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

package loader

import (
	"strconv"
	"strings"

	interp "github.com/compose-spec/compose-go/interpolation"
	"github.com/pkg/errors"
)

var interpolateTypeCastMapping = map[interp.Path]interp.Cast{
	servicePath("configs", interp.PathMatchList, "mode"):             toInt,
	servicePath("secrets", interp.PathMatchList, "mode"):             toInt,
	servicePath("healthcheck", "retries"):                            toInt,
	servicePath("healthcheck", "disable"):                            toBoolean,
	servicePath("deploy", "replicas"):                                toInt,
	servicePath("deploy", "update_config", "parallelism"):            toInt,
	servicePath("deploy", "update_config", "max_failure_ratio"):      toFloat,
	servicePath("deploy", "rollback_config", "parallelism"):          toInt,
	servicePath("deploy", "rollback_config", "max_failure_ratio"):    toFloat,
	servicePath("deploy", "restart_policy", "max_attempts"):          toInt,
	servicePath("deploy", "placement", "max_replicas_per_node"):      toInt,
	servicePath("ports", interp.PathMatchList, "target"):             toInt,
	servicePath("ports", interp.PathMatchList, "published"):          toInt,
	servicePath("ulimits", interp.PathMatchAll):                      toInt,
	servicePath("ulimits", interp.PathMatchAll, "hard"):              toInt,
	servicePath("ulimits", interp.PathMatchAll, "soft"):              toInt,
	servicePath("privileged"):                                        toBoolean,
	servicePath("read_only"):                                         toBoolean,
	servicePath("stdin_open"):                                        toBoolean,
	servicePath("tty"):                                               toBoolean,
	servicePath("volumes", interp.PathMatchList, "read_only"):        toBoolean,
	servicePath("volumes", interp.PathMatchList, "volume", "nocopy"): toBoolean,
	iPath("networks", interp.PathMatchAll, "external"):               toBoolean,
	iPath("networks", interp.PathMatchAll, "internal"):               toBoolean,
	iPath("networks", interp.PathMatchAll, "attachable"):             toBoolean,
	iPath("volumes", interp.PathMatchAll, "external"):                toBoolean,
	iPath("secrets", interp.PathMatchAll, "external"):                toBoolean,
	iPath("configs", interp.PathMatchAll, "external"):                toBoolean,
}

func iPath(parts ...string) interp.Path {
	return interp.NewPath(parts...)
}

func servicePath(parts ...string) interp.Path {
	return iPath(append([]string{"services", interp.PathMatchAll}, parts...)...)
}

func toInt(value string) (interface{}, error) {
	return strconv.Atoi(value)
}

func toFloat(value string) (interface{}, error) {
	return strconv.ParseFloat(value, 64)
}

// should match http://yaml.org/type/bool.html
func toBoolean(value string) (interface{}, error) {
	switch strings.ToLower(value) {
	case "y", "yes", "true", "on":
		return true, nil
	case "n", "no", "false", "off":
		return false, nil
	default:
		return nil, errors.Errorf("invalid boolean: %s", value)
	}
}
