#!/bin/bash

set -euo pipefail

cd $(dirname "$0")/..

# Create a temporary directory for outputs
TEMP_DIR=$(mktemp -d)

trap "rm -rf $TEMP_DIR" EXIT

rm -f \
   web/src/view.d.ts \
   pkg/webview/log.pb.go \
   pkg/webview/view.pb.go \
   pkg/webview/view_grpc.pb.go \
   pkg/webview/view.swagger.json

# Build the webview-proto stage and extract outputs

docker buildx build -f ./scripts/codegen-webview.Dockerfile --target=webview-proto-output --output=type=local,dest=$TEMP_DIR .

# Copy the generated files back to their original locations

cp $TEMP_DIR/go/src/github.com/tilt-dev/tilt/pkg/webview/log.pb.go pkg/webview/

cp $TEMP_DIR/go/src/github.com/tilt-dev/tilt/pkg/webview/view.pb.go pkg/webview/

cp $TEMP_DIR/go/src/github.com/tilt-dev/tilt/pkg/webview/view_grpc.pb.go pkg/webview/

cp $TEMP_DIR/go/src/github.com/tilt-dev/tilt/pkg/webview/view.swagger.json pkg/webview/

# Build the proto-ts stage and extract outputs

docker buildx build -f ./scripts/codegen-webview.Dockerfile --target=proto-ts-output --output=type=local,dest=$TEMP_DIR .

# Copy the generated TypeScript definitions back

cp $TEMP_DIR/go/src/github.com/tilt-dev/tilt/web/src/view.d.ts web/src/
