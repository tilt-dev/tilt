#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

if [ "${BASH_VERSINFO:-0}" -lt 5 ]; then
  >&2 printf "This script requires Bash 5.0+.\n\nOn macOS, run brew install bash and relaunch your terminal.\n"
  exit 2
fi

if [[ -n "${CODEGEN_USER-}" ]]; then
    useradd "$CODEGEN_USER"
fi

GOPATH=$(go env GOPATH)
export GOPATH

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

if [[ "$(pwd)" != "${GOPATH}"* ]]; then
    >&2 echo "ERROR: update-codegen.sh does not work correctly outside of GOPATH: $GOPATH $(pwd)"
    exit 1
fi

bash "${CODEGEN_PKG}/generate-internal-groups.sh" "deepcopy,defaulter,openapi" \
  github.com/tilt-dev/tilt/pkg github.com/tilt-dev/tilt/pkg/apis github.com/tilt-dev/tilt/pkg/apis \
  "core:v1alpha1" \
  --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
  --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"
