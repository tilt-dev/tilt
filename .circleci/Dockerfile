FROM circleci/golang:1.14

RUN go get -u github.com/kisielk/errcheck

RUN go get -u github.com/google/wire/cmd/wire

RUN go get -u golang.org/x/tools/cmd/goimports

RUN sudo apt-get install default-jdk

USER root

RUN curl -sL https://deb.nodesource.com/setup_12.x | bash -

RUN apt-get install -y nodejs

USER circleci

RUN sudo npm install -g firebase-tools

RUN firebase setup:emulators:firestore

USER root

RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/kustomize.tar.gz  https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.4.0/kustomize_v3.4.0_linux_amd64.tar.gz \
  && tar -xz -C /tmp -f /tmp/kustomize.tar.gz \
  && mv /tmp/kustomize /usr/bin/kustomize

RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/helm.tar.gz https://get.helm.sh/helm-v2.16.1-linux-amd64.tar.gz \
  && tar -xz -C /tmp -f /tmp/helm.tar.gz \
  && mv /tmp/linux-amd64/helm /usr/bin/helm2

RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/helm.tar.gz  https://get.helm.sh/helm-v3.0.0-linux-amd64.tar.gz \
  && tar -xz -C /tmp -f /tmp/helm.tar.gz \
  && mv /tmp/linux-amd64/helm /usr/bin/helm3

USER circleci

RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/google-cloud-sdk.tar.gz https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-265.0.0-linux-x86_64.tar.gz \
  && tar -xz -C /tmp -f /tmp/google-cloud-sdk.tar.gz

RUN /tmp/google-cloud-sdk/install.sh

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.23.8

RUN curl -o- -L https://yarnpkg.com/install.sh | bash -s -- --version 1.19.0

ENV PATH="${PATH}:/tmp/google-cloud-sdk/bin"
ENV PATH="${PATH}:/home/circleci/.yarn/bin"
