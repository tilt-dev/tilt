#!/bin/bash
#
# Adapted from:
# https://github.com/kubernetes-sigs/kind/commits/master/site/static/examples/kind-with-registry.sh
#
# Copyright 2020 The Kubernetes Project
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

set -oe errexit

# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="integration"

# default registry name and port
reg_name='kind-registry'
reg_port='5000'

echo "> initializing Kind cluster: ${KIND_CLUSTER_NAME}"
# create registry container unless it already exists
running="$(docker inspect -f '{{.State.Running}}' "${reg_name}" 2>/dev/null || true)"
if [ "${running}" != 'true' ]; then
  docker run \
    -d --restart=always -p "${reg_port}:5000" --name "${reg_name}" \
    registry:2
fi

# create a cluster with the local registry enabled in containerd
cat <<EOF | kind create cluster --name "${KIND_CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry:${reg_port}"]
    endpoint = ["http://registry:${reg_port}"]
EOF

echo "> port-forwarding k8s API server"
/go/portforward.sh start
APISERVER_PORT=$(kubectl config view -o jsonpath='{.clusters[].cluster.server}' | cut -d: -f 3 -)
/go/portforward.sh -wait $APISERVER_PORT
kubectl get nodes # make sure it worked

echo "> port-forwarding local registry"
/go/portforward.sh -wait $reg_port

echo "> annotating nodes"
# add the registry to /etc/hosts on each node
ip_fmt='{{.NetworkSettings.IPAddress}}'
cmd="echo $(docker inspect -f "${ip_fmt}" "${reg_name}") registry >> /etc/hosts"

# and annotate each node with registry info (for Tilt to detect)
for node in $(kind get nodes --name "${KIND_CLUSTER_NAME}"); do
  docker exec "${node}" sh -c "${cmd}"
  kubectl annotate node "${node}" \
          tilt.dev/registry=localhost:${reg_port} \
          tilt.dev/registry-from-cluster=registry:${reg_port}
done

echo "> running integration tests"
make integration
