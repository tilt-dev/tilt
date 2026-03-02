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

package validation

import (
	"fmt"
	"net"
	"strings"

	"github.com/compose-spec/compose-go/v2/tree"
)

type checkerFunc func(value any, p tree.Path) error

var checks = map[tree.Path]checkerFunc{
	"volumes.*":                       checkVolume,
	"configs.*":                       checkFileObject("file", "environment", "content"),
	"secrets.*":                       checkFileObject("file", "environment"),
	"services.*.ports.*":              checkIPAddress,
	"services.*.develop.watch.*.path": checkPath,
	"services.*.deploy.resources.reservations.devices.*": checkDeviceRequest,
	"services.*.gpus.*": checkDeviceRequest,
}

func Validate(dict map[string]any) error {
	return check(dict, tree.NewPath())
}

func check(value any, p tree.Path) error {
	for pattern, fn := range checks {
		if p.Matches(pattern) {
			return fn(value, p)
		}
	}
	switch v := value.(type) {
	case map[string]any:
		for k, v := range v {
			err := check(v, p.Next(k))
			if err != nil {
				return err
			}
		}
	case []any:
		for _, e := range v {
			err := check(e, p.Next("[]"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func checkFileObject(keys ...string) checkerFunc {
	return func(value any, p tree.Path) error {
		v := value.(map[string]any)
		count := 0
		for _, s := range keys {
			if _, ok := v[s]; ok {
				count++
			}
		}
		if count > 1 {
			return fmt.Errorf("%s: %s attributes are mutually exclusive", p, strings.Join(keys, "|"))
		}
		if count == 0 {
			if _, ok := v["driver"]; ok {
				// User specified a custom driver, which might have it's own way to set content
				return nil
			}
			if _, ok := v["external"]; !ok {
				return fmt.Errorf("%s: one of %s must be set", p, strings.Join(keys, "|"))
			}
		}
		return nil
	}
}

func checkPath(value any, p tree.Path) error {
	v := value.(string)
	if v == "" {
		return fmt.Errorf("%s: value can't be blank", p)
	}
	return nil
}

func checkDeviceRequest(value any, p tree.Path) error {
	v := value.(map[string]any)
	_, hasCount := v["count"]
	_, hasIDs := v["device_ids"]
	if hasCount && hasIDs {
		return fmt.Errorf(`%s: "count" and "device_ids" attributes are exclusive`, p)
	}
	return nil
}

func checkIPAddress(value any, p tree.Path) error {
	if v, ok := value.(map[string]any); ok {
		ip, ok := v["host_ip"]
		if ok && net.ParseIP(ip.(string)) == nil {
			return fmt.Errorf("%s: invalid ip address: %s", p, ip)
		}
	}
	return nil
}
