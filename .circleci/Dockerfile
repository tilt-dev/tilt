FROM circleci/golang:1.12

RUN go get -u github.com/kisielk/errcheck

RUN go get -u github.com/google/wire/cmd/wire

RUN go get -u sigs.k8s.io/kustomize/cmd/kustomize

RUN go get -u golang.org/x/tools/cmd/goimports

RUN sudo apt-get install default-jdk

RUN sudo apt-get install npm

RUN sudo npm install -g firebase-tools

RUN firebase setup:emulators:firestore

USER root
RUN curl --silent --show-error --location --fail --retry 3 --output /tmp/helm.tar.gz  https://storage.googleapis.com/kubernetes-helm/helm-v2.12.1-linux-amd64.tar.gz \
  && tar -xz -C /tmp -f /tmp/helm.tar.gz \
  && mv /tmp/linux-amd64/helm /usr/bin/helm

USER circleci
