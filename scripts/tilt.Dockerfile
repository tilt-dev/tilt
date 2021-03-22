# Builds a Docker image with:
# - tilt
# - ctlptl
# - docker
# - kubectl
# - kind
# - socat
# - git
#
# Good base image for anyone that wants to use ctlptl in a CI environment
# to set up a one-time-use cluster, and use `tilt ci` to run the tests.
#
# Built with goreleaser.

FROM tiltdev/ctlptl

# Tilt's extension downloader requires git
RUN apt update && apt install -y git

# install circleci helpers from
# https://github.com/tilt-dev/kind-local/tree/master/.circleci
# for backwards-compatibility. These are largely obsoleted by the ctlptl image.
COPY --from=tiltdev/circleci-kind:v1.4.0 /usr/local/bin/start-portforward-service.sh /usr/local/bin/
COPY --from=tiltdev/circleci-kind:v1.4.0 /usr/local/bin/portforward.sh /usr/local/bin/
COPY --from=tiltdev/circleci-kind:v1.4.0 /usr/local/bin/with-kind-cluster.sh /usr/local/bin/

COPY tilt /usr/local/bin/tilt
