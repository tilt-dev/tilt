# An image with cross-compilation toolchains.
#
# The goal here is to both:
# 1) Better leverage OS-specific C headers
# 2) Be able to do releases from a CI job

FROM docker/tilt-golang-cross:1.18.0-1

RUN apt-get update && \
    apt-get install -y -q --no-install-recommends \
        apt-transport-https \
        ca-certificates \
        curl \
        gnupg-agent \
        software-properties-common \
    && rm -rf /var/lib/apt/lists/*

# Install docker
# Adapted from https://github.com/CircleCI-Public/cimg-base/blob/main/22.04/Dockerfile#L97-L110
# Changed to work with debian via https://docs.docker.com/engine/install/debian/
ENV DOCKER_VERSION 5:20.10.14~3-0~debian-
RUN set -exu && \
    apt-get update && apt-get install -y \
        ca-certificates \
        curl \
        gnupg \
        lsb-release && \
    mkdir -p /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian \
      $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce=${DOCKER_VERSION}$( lsb_release -cs ) docker-ce-cli=${DOCKER_VERSION}$( lsb_release -cs ) containerd.io && \
    docker --version && \
    rm -rf /var/lib/apt/lists/*

ENV GORELEASER_VERSION=v1.6.3
RUN set -exu \
  && URL="https://github.com/goreleaser/goreleaser/releases/download/${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz" \
  && echo goreleaser URL: $URL \
  && curl --silent --show-error --location --fail --retry 3 --output /tmp/goreleaser.tar.gz "${URL}" \
  && tar -C /tmp -xzf /tmp/goreleaser.tar.gz \
  && mv /tmp/goreleaser /usr/bin/ \
  && goreleaser --version

RUN curl -sL https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - && \
    echo "deb https://dl.yarnpkg.com/debian/ stable main" | tee /etc/apt/sources.list.d/yarn.list && \
    curl -sL https://deb.nodesource.com/setup_16.x | bash - && \
    apt install -y -q --no-install-recommends \
      nodejs \
      yarn \
    && rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/Homebrew/brew /home/linuxbrew/.linuxbrew
ENV PATH=/home/linuxbrew/.linuxbrew/bin:$PATH

RUN mkdir -p ~/.windmill
