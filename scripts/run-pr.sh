#!/bin/bash
# run-pr.sh — Build & run a tilt PR in a throwaway container, with the
# tilt-example-html/0-base project loaded.
#
# Usage: scripts/run-pr.sh <pr-number>
set -euo pipefail

PR_NUMBER="${1:?Usage: $0 <pr-number>}"

# ---------- resolve PR via gh -----------------------------------------------
echo "==> Looking up PR #${PR_NUMBER}..."
PR_JSON=$(gh pr view "$PR_NUMBER" --repo tilt-dev/tilt --json title,headRefName,headRepository,headRepositoryOwner)
PR_TITLE=$(echo "$PR_JSON" | jq -r .title)
PR_BRANCH=$(echo "$PR_JSON" | jq -r .headRefName)
PR_OWNER=$(echo "$PR_JSON" | jq -r .headRepositoryOwner.login)
PR_REPO=$(echo "$PR_JSON" | jq -r .headRepository.name)
CLONE_URL="https://github.com/${PR_OWNER}/${PR_REPO}.git"

echo "    Title:  ${PR_TITLE}"
echo "    Branch: ${PR_OWNER}:${PR_BRANCH}"
echo "    Repo:   ${CLONE_URL}"

IMAGE_NAME="tilt-pr-${PR_NUMBER}"
CONTAINER_NAME="tilt-pr-${PR_NUMBER}-test"
PORT=10350

cleanup() {
    echo ""
    echo "==> Cleaning up..."
    docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
}
trap cleanup EXIT

# ---------- check kubeconfig -----------------------------------------------
KUBE_CONTEXT=$(kubectl config current-context)
if [[ "$KUBE_CONTEXT" != *"docker-desktop"* ]]; then
    echo "Error: kubeconfig context is ${KUBE_CONTEXT}" >&2
    echo "       This script only supports local clusters running in Docker Desktop." >&2
    exit 1
fi

K8S_SERVER=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}')
if [ -z "$K8S_SERVER" ]; then
    echo "Error: could not determine k8s server from kubeconfig." >&2
    exit 1
fi

K8S_HOST=$(echo "$K8S_SERVER" | sed -E 's|https?://([^:]+):.*|\1|')
K8S_PORT=$(echo "$K8S_SERVER" | sed -E 's|https?://[^:]+:([0-9]+).*|\1|')

if [[ "$K8S_HOST" != "127.0.0.1" && "$K8S_HOST" != "localhost" ]]; then
    echo "Error: kubeconfig cluster server points to ${K8S_SERVER}" >&2
    echo "       Expected a cluster at localhost. This script only supports local clusters." >&2
    exit 1
fi

echo "==> Detected k8s API at ${K8S_HOST}:${K8S_PORT}"

# ---------- build -----------------------------------------------------------
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true

echo "==> Building tilt from PR #${PR_NUMBER} (this may take a few minutes)..."

docker build -t "$IMAGE_NAME" - <<DOCKERFILE
# --- Stage 1: build web assets with Node ---
FROM node:20 AS web
RUN git clone ${CLONE_URL} /tilt
WORKDIR /tilt
RUN git checkout ${PR_BRANCH}
WORKDIR /tilt/web
RUN corepack enable && yarn install --immutable && yarn build

# --- Stage 2: build tilt binary with Go ---
FROM golang:1.25
RUN apt-get update && apt-get install -y socat && rm -rf /var/lib/apt/lists/*
RUN git clone ${CLONE_URL} /tilt
WORKDIR /tilt
RUN git checkout ${PR_BRANCH}
# Overlay the freshly-built web assets into the embed directory
COPY --from=web /tilt/web/build/ pkg/assets/build/
RUN go install -mod vendor ./cmd/tilt/...

# Clone the example project that tilt will run against
RUN git clone https://github.com/tilt-dev/tilt-example-html.git /tilt-example-html

WORKDIR /tilt-example-html/0-base
DOCKERFILE

# ---------- run --------------------------------------------------------------
echo "==> Starting tilt (web UI at http://localhost:${PORT})..."

# Open the browser after tilt has a moment to start
(sleep 5 && open "http://localhost:${PORT}") &

# Inside the container:
#  - socat forwards 127.0.0.1:<k8s-port> → host.docker.internal:<k8s-port>
#    so the kubeconfig's localhost reference reaches the host's k8s API.
#  - tilt runs against the example project in the foreground.
docker run -it --rm \
    --name "$CONTAINER_NAME" \
    -p "${PORT}:${PORT}" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "${HOME}/.kube:/root/.kube:ro" \
    "$IMAGE_NAME" \
    bash -c "socat TCP-LISTEN:${K8S_PORT},fork,reuseaddr,bind=127.0.0.1 TCP:host.docker.internal:${K8S_PORT} & exec tilt up --host 0.0.0.0 --web-mode=prod"
