# Builds a Docker image with:
# - tilt
# - Docker
# - kubectl
# - git
# and scripts you need to run integration tests with tilt.
#
# Built with goreleaser.

FROM tiltdev/circleci-kind:v1.4.0

RUN apt update && apt install -y git

COPY tilt /usr/local/bin/tilt
