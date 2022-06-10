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

FROM docker/tilt-ctlptl

# Tilt's extension downloader requires git
RUN apt update && apt install -y git

COPY tilt /usr/local/bin/tilt
