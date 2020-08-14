# An image with cross-compilation toolchains.
#
# The goal here is to both:
# 1) Better leverage OS-specific C headers
# 2) Be able to do releases from a CI job

FROM dockercore/golang-cross:1.13.15

ENV GORELEASER_VERSION=0.127.0
ENV GORELEASER_SHA=bf7e0f34d1d46041f302a0dd773a5c70ff7476c147d3a30659a5a11e823bccbd

ENV GORELEASER_DOWNLOAD_FILE=goreleaser_Linux_x86_64.tar.gz
ENV GORELEASER_DOWNLOAD_URL=https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/${GORELEASER_DOWNLOAD_FILE}

RUN apt-get update && \
    apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg-agent \
    software-properties-common

# Install docker
# Adapted from https://github.com/circleci/circleci-images/blob/staging/shared/images/Dockerfile-basic.template
# Check https://download.docker.com/linux/static/stable/x86_64/ for latest versions
ENV DOCKER_VERSION=19.03.5
RUN set -exu \
  && DOCKER_URL="https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}.tgz" \
  && echo Docker URL: $DOCKER_URL \
  && curl --silent --show-error --location --fail --retry 3 --output /tmp/docker.tgz "${DOCKER_URL}" \
  && ls -lha /tmp/docker.tgz \
  && tar -xz -C /tmp -f /tmp/docker.tgz \
  && mv /tmp/docker/* /usr/bin \
  && rm -rf /tmp/docker /tmp/docker.tgz \
  && which docker \
  && (docker version || true)

ENV GORELEASER_VERSION=v0.141.0
RUN set -exu \
  && URL="https://github.com/goreleaser/goreleaser/releases/download/${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz" \
  && echo goreleaser URL: $URL \
  && curl --silent --show-error --location --fail --retry 3 --output /tmp/goreleaser.tar.gz "${URL}" \
  && tar -C /tmp -xzf /tmp/goreleaser.tar.gz \
  && mv /tmp/goreleaser /usr/bin/ \
  && goreleaser --version

ENTRYPOINT ["goreleaser"]
CMD ["-h"]
