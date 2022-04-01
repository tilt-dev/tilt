FROM cimg/go:1.18-node

# --- DEPENDENCIES ---
USER root

# NOTE: cimg/go already includes Docker Compose v2
RUN curl -L "https://github.com/docker/compose/releases/download/1.29.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose-v1 \
    && chmod +x /usr/local/bin/docker-compose-v1

RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/kustomize.tar.gz  https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.9.2/kustomize_v3.9.2_linux_amd64.tar.gz \
  && tar -xz -C /tmp -f /tmp/kustomize.tar.gz \
  && mv /tmp/kustomize /usr/bin/kustomize \
  && rm -f /tmp/kustomize.tar.gz

RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/helm.tar.gz  https://get.helm.sh/helm-v3.8.1-linux-amd64.tar.gz \
  && tar -xz -C /tmp -f /tmp/helm.tar.gz \
  && mv /tmp/linux-amd64/helm /usr/bin/helm3 \
  && rm -f /tmp/helm.tar.gz

# --- GO UTILITIES / LINTERS ---
USER circleci
RUN go install github.com/google/wire/cmd/wire@latest \
    && go install golang.org/x/tools/cmd/goimports@latest \
    && go clean -cache -modcache

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.45.2
