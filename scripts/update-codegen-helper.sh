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

GOPATH=$(go env GOPATH)
export GOPATH

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

if [[ "$(pwd)" != "${GOPATH}"* ]]; then
    >&2 echo "ERROR: update-codegen.sh does not work correctly outside of GOPATH: $GOPATH $(pwd)"
    exit 1
fi

git config --global --add safe.directory /go/src/github.com/tilt-dev/tilt

sed -i "s/types.go/*_types.go/g" "${CODEGEN_PKG}/kube_codegen.sh"
source "${CODEGEN_PKG}/kube_codegen.sh"
sed -i "s/[*]_types.go/types.go/g" "${CODEGEN_PKG}/kube_codegen.sh"

echo "Generating code..."
rm -fR pkg/apis/**/zz_generated*
kube::codegen::gen_helpers \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
  ./pkg/apis

rm -fR pkg/openapi
VIOLATIONS_ORIG_FILE=$(mktemp)
VIOLATIONS_FILE=$(mktemp)
kube::codegen::gen_openapi \
  --output-pkg github.com/tilt-dev/tilt/pkg/openapi \
  --output-dir ./pkg/openapi \
  --output-model-name-file zz_generated.model_name.go \
  --report-filename "${VIOLATIONS_ORIG_FILE}" \
  --update-report \
  --boilerplate "${SCRIPT_ROOT}"/hack/openapi-boilerplate.go.txt \
  ./pkg/apis

# add a sentinel line at the end of the file.
# this ensures grep -v doesn't remove all the warnings.
echo "
end" >> "$VIOLATIONS_ORIG_FILE"

echo "Checking violations..."
cat "$VIOLATIONS_ORIG_FILE" | \
  grep -v "API rule violation.*k8s.io" | \
  grep -v list_type_missing > "$VIOLATIONS_FILE"

VIOLATIONS=$(grep "API rule violation" "$VIOLATIONS_FILE" || echo -n "ok")
if [[ "$VIOLATIONS" != "ok" ]]; then
    echo "ERROR: found API rule violations in tilt code"
    echo "$VIOLATIONS"
    exit 1
fi

FIXUPS=(
./pkg/openapi
./pkg/openapi/zz_generated.openapi.go
./pkg/apis/core/v1alpha1/zz_generated.conversion.go
./pkg/apis/core/v1alpha1/zz_generated.model_name.go
./pkg/apis/core/v1alpha1/generated.proto
./pkg/apis/core/v1alpha1/zz_generated.defaults.go
./pkg/apis/core/v1alpha1/generated.pb.go
./pkg/apis/core/v1alpha1/zz_generated.deepcopy.go
./pkg/apis/core/zz_generated.deepcopy.go
)

USER_ID=$(id -u)

# uid = 0 means we're running in docker desktop with
# a fake user.
if [[ "$USER_ID" != "0" && "$CODEGEN_UID" != "$USER_ID" ]]; then
    groupadd --gid "$CODEGEN_GID" codegen-user
    useradd --uid "$CODEGEN_UID" -g codegen-user codegen-user

    for f in "${FIXUPS[@]}"; do
        if [ -d "$f" ]; then
            chmod 775 "$f"
        else
            chmod 664 "$f"
        fi
        chown codegen-user:codegen-user "$f"
    done
fi
