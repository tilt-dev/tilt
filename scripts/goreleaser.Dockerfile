# Builds a Docker image with:
# - tilt
# - Docker
# - kubectl
# and scripts you need to run integration tests with tilt.
#
# Built with goreleaser.

FROM tiltdev/circleci-kind:v1.2.0

COPY tilt /usr/local/bin/tilt
