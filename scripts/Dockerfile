# Builds a Docker image with:
# - tilt
# - Docker
# - kubectl
# and scripts you need to run integration tests with tilt.
#
# Build with:
# docker build -t tiltdev/circleci-tilt:v0.13.0 --build-arg TILT_VERSION=0.13.0 .
#
# TODO(nick): Auto-build this Dockerfile on each new tilt release.

FROM tiltdev/circleci-kind:v1.0.0

ARG TILT_VERSION
RUN set -ex && \
  curl -fsSL https://github.com/windmilleng/tilt/releases/download/v$TILT_VERSION/tilt.$TILT_VERSION.linux.x86_64.tar.gz | tar -xzv tilt && \
  mv tilt /usr/local/bin/tilt
