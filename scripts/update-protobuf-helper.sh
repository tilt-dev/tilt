#!/bin/bash
#
# Generates protobufs for Tilt API Server objects.
# Intended to be run inside the container created by protobuf-helper.dockerfile

set -ex

echo $GOPATH
cd /go/src/github.com/tilt-dev/tilt
go-to-protobuf -h ./hack/boilerplate.go.txt \
               --packages github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1 \
               --proto-import /go/src/github.com/tilt-dev/tilt/vendor \
               --apimachinery-packages "-k8s.io/apimachinery/pkg/util/intstr,-k8s.io/apimachinery/pkg/api/resource,-k8s.io/apimachinery/pkg/runtime/schema,-k8s.io/apimachinery/pkg/runtime,-k8s.io/apimachinery/pkg/apis/meta/v1"

