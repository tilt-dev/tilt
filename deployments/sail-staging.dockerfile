# The sail-staging dockerfile looks very different than the sail dockerfile
# because sail-staging always uses precompiled JS.

FROM golang:1.12
WORKDIR /go/src/github.com/windmilleng/tilt

RUN curl -sL https://deb.nodesource.com/setup_11.x | bash -
RUN curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | apt-key add -
RUN echo "deb https://dl.yarnpkg.com/debian/ stable main" | tee /etc/apt/sources.list.d/yarn.list
RUN apt update && apt install -y nodejs yarn

# Install JS deps
ADD web/package.json web/package.json
RUN cd web && yarn install

# Compile JS code
ADD Makefile Makefile
ADD web web
RUN make build-js

# Build the Go code
ADD . .
RUN make install-sail

ENTRYPOINT sail --web-mode=precompiled