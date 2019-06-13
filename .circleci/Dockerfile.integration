FROM golang:1.12

# Install docker
# Adapted from https://github.com/circleci/circleci-images/blob/staging/shared/images/Dockerfile-basic.template
RUN set -ex \
  && export DOCKER_VERSION=$(curl --silent --fail --retry 3 https://download.docker.com/linux/static/stable/x86_64/ | grep -o -e 'docker-[.0-9]*-ce\.tgz' | sort -r | head -n 1) \
  && DOCKER_URL="https://download.docker.com/linux/static/stable/x86_64/${DOCKER_VERSION}" \
  && echo Docker URL: $DOCKER_URL \
  && curl --silent --show-error --location --fail --retry 3 --output /tmp/docker.tgz "${DOCKER_URL}" \
  && ls -lha /tmp/docker.tgz \
  && tar -xz -C /tmp -f /tmp/docker.tgz \
  && mv /tmp/docker/* /usr/bin \
  && rm -rf /tmp/docker /tmp/docker.tgz \
  && which docker \
  && (docker version || true)

# Install docker-compose
RUN curl -L "https://github.com/docker/compose/releases/download/1.23.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose \
  && chmod a+x /usr/local/bin/docker-compose \
  && docker-compose version

# Install kubectl client
RUN apt update && apt install -y apt-transport-https \
  && curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - \
  && touch /etc/apt/sources.list.d/kubernetes.list \
  && echo "deb http://apt.kubernetes.io/ kubernetes-xenial main" | tee -a /etc/apt/sources.list.d/kubernetes.list \
  && apt update && apt install -y kubectl

# Install gcloud.
# Adapted from https://github.com/GoogleCloudPlatform/cloud-sdk-docker/blob/master/Dockerfile
ENV CLOUD_SDK_VERSION=219.0.1
RUN apt-get -qqy update && apt-get install -qqy \
        curl \
        gcc \
        python-dev \
        python-setuptools \
        apt-transport-https \
        lsb-release \
        openssh-client \
        git \
        gnupg \
    && easy_install -U pip && \
    pip install -U crcmod   && \
    export CLOUD_SDK_REPO="cloud-sdk-$(lsb_release -c -s)" && \
    echo "deb https://packages.cloud.google.com/apt $CLOUD_SDK_REPO main" > /etc/apt/sources.list.d/google-cloud-sdk.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add - && \
    apt-get update && \
    apt-get install -y google-cloud-sdk=${CLOUD_SDK_VERSION}-0 && \
    gcloud config set core/disable_usage_reporting true && \
    gcloud config set component_manager/disable_update_check true && \
    gcloud config set metrics/environment github_docker_image && \
    gcloud --version

# Install cluster script, which downloads the necessary containers.
# The dind-based kubernetes cluster uses socat to do port-forwarding
ENV KUBEADM_SHA=30a2033581adf53161fe1cdc76f1550193927db4
ADD https://raw.githubusercontent.com/kubernetes-sigs/kubeadm-dind-cluster/${KUBEADM_SHA}/fixed/dind-cluster-v1.12.sh ./dind-cluster.sh
ADD https://raw.githubusercontent.com/kubernetes-sigs/kubeadm-dind-cluster/${KUBEADM_SHA}/build/portforward.sh .
RUN apt install -y curl ca-certificates git liblz4-tool rsync socat \
  && chmod a+x /go/dind-cluster.sh \
  && chmod a+x /go/portforward.sh

# install gotestsum
RUN go get gotest.tools/gotestsum

RUN apt install -y jq
