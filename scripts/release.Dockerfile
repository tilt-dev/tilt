# An image with cross-compilation toolchains.
#
# The goal here is to both:
# 1) Better leverage OS-specific C headers
# 2) Be able to do releases from a CI job

# osxcross contains the MacOSX cross toolchain for xx
FROM crazymax/osxcross:11.3-debian AS osxcross

FROM golang:1.23-bullseye as musl-cross
WORKDIR /musl
# https://more.musl.cc/GCC-MAJOR-VERSION/HOST-ARCH-linux-musl/CROSS-ARCH-linux-musl-cross.tgz
RUN curl -sf https://more.musl.cc/11/x86_64-linux-musl/aarch64-linux-musl-cross.tgz | tar zxf -
RUN curl -sf https://more.musl.cc/11/x86_64-linux-musl/x86_64-linux-musl-cross.tgz | tar zxf -

FROM golang:1.23-bullseye

RUN apt-get update && \
    apt-get install -y -q --no-install-recommends \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    gnupg-agent \
    software-properties-common \
    clang \
    lld \
    libc6-dev \
    libltdl-dev \
    zlib1g-dev \
    g++-aarch64-linux-gnu \
    gcc-aarch64-linux-gnu \
    g++-arm-linux-gnueabi \
    gcc-arm-linux-gnueabi \
    g++-mingw-w64 \
    gcc-mingw-w64 \
    parallel \
    && rm -rf /var/lib/apt/lists/*

# Install docker
RUN set -exu \
  # Add Docker's official GPG key:
  && install -m 0755 -d /etc/apt/keyrings \
  && curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc \
  && chmod a+r /etc/apt/keyrings/docker.asc \
  # Add the repository to Apt sources: 
  && echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null \
    && apt update \
  && apt install -y docker-ce-cli=5:25.0.3-1~debian.11~bullseye docker-buildx-plugin

ENV GORELEASER_VERSION=v2.4.4
RUN set -exu \
  && URL="https://github.com/goreleaser/goreleaser/releases/download/${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz" \
  && echo goreleaser URL: $URL \
  && curl --silent --show-error --location --fail --retry 3 --output /tmp/goreleaser.tar.gz "${URL}" \
  && tar -C /tmp -xzf /tmp/goreleaser.tar.gz \
  && mv /tmp/goreleaser /usr/bin/ \
  && goreleaser --version

RUN mkdir -p /etc/apt/keyrings && \
    curl -sL https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add - && \
    echo "deb https://dl.yarnpkg.com/debian/ stable main" | tee /etc/apt/sources.list.d/yarn.list && \
    curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_22.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list && \
    apt update && \
    apt install -y -q --no-install-recommends \
      nodejs \
      yarn \
    && rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/Homebrew/brew /home/linuxbrew/.linuxbrew
COPY --from=osxcross /osxcross /osxcross
COPY --from=musl-cross /musl /musl

ENV PATH=/home/linuxbrew/.linuxbrew/bin:/osxcross/bin:/musl/aarch64-linux-musl-cross/bin:/musl/x86_64-linux-musl-cross/bin:$PATH
ENV LD_LIBRARY_PATH=/osxcross/lib:$LD_LIBRARY_PATH

RUN mkdir -p ~/.windmill
