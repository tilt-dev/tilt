# Uses Node 24
FROM cimg/go:1.27-node

# --- DEPENDENCIES ---
USER root

# NOTE: cimg/go already includes Docker Compose v2, but it's not always up-to-date
ARG COMPOSE_VERSION="5.5.1"
RUN mkdir -p "${HOME}/.docker/cli-plugins" \
    && curl -sSL "https://github.com/docker/compose/releases/download/v${COMPOSE_VERSION}/docker-compose-linux-$(uname -m)" -o "${HOME}/.docker/cli-plugins/docker-compose" \
    && chmod +x "${HOME}/.docker/cli-plugins/docker-compose" \
    && docker compose version --short | grep -q -F "${COMPOSE_VERSION}"

ARG KUSTOMIZE_VERSION="5.8.1"
RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/kustomize.tar.gz "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" \
  && tar -xz -C /tmp -f /tmp/kustomize.tar.gz \
  && mv /tmp/kustomize /usr/bin/kustomize \
  && rm -f /tmp/kustomize.tar.gz \
  && kustomize version --short | grep -q -F "${KUSTOMIZE_VERSION}"

ARG HELM_VERSION="4.2.4"
RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/helm.tar.gz "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" \
  && tar -xz -C /tmp -f /tmp/helm.tar.gz \
  && mv /tmp/linux-amd64/helm /usr/bin/helm \
  && helm version --short | grep -q -F "${HELM_VERSION}"

# --- GO UTILITIES / LINTERS ---
USER circleci
RUN go install github.com/google/wire/cmd/wire@latest \
    && go install golang.org/x/tools/cmd/goimports@latest \
    && go clean -cache -modcache

RUN curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.13.2
