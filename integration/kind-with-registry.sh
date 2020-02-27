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

set -o errexit

HELPTEXT="
Run this script to start a Kind cluster with a local image registry enabled,
with its nodes annotated such that Tilt can auto-detecte the registry.

Set KIND_CLUSTER_NAME to specify cluster name; otherwise, uses the default ('kind').
"
DO_INIT=true
DO_ANNOTATE=true

while getopts ":s:h" opt; do
  case $opt in
    s)
      # Secret option for use in CI; specify [s]tage to run a specific stage of this script. Options:
      #     `init`: initialize the cluster+registry
      #     `annotate`: annotate the node(s) of the existing Kind cluster
      if [ ${OPTARG} == 'init' ]; then
        DO_ANNOTATE='false'
      elif [ ${OPTARG} == 'annotate' ]; then
        DO_INIT='false'
      else
        echo "Invalid value ${OPTARG} for flag -s. Valid values:"
        echo "\tinit: initialize the cluster+registry"
        echo "\tannotate: annotate the node(s) of the existing Kind cluster"
        exit 1
      fi
      ;;
    h)
      echo "$HELPTEXT"
      ;;
    \?)
      echo "Invalid option: -$OPTARG"
      echo "$HELPTEXT"
      exit 1
      ;;
    :)
      echo "Option -$OPTARG requires an argument. Valid values:"
        echo "\tinit: initialize the cluster+registry"
        echo "\tannotate: annotate the node(s) of the existing Kind cluster"
      exit 1
      ;;
  esac
done

# desired cluster name; default is "kind"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"

# default registry name and port
reg_name='kind-registry'
reg_port='5000'

# STEP 1: Init cluster
if [ "${DO_INIT}" == 'true' ]; then
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

fi

# STEP 2: Annotate node(s)
if [ "${DO_ANNOTATE}" == 'true' ]; then
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
fi
