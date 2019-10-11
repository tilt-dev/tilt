import React from "react"
import ReactDOM from "react-dom"
import LogPane from "./LogPane"
import renderer from "react-test-renderer"

it("renders without crashing", () => {
  let div = document.createElement("div")
  Element.prototype.scrollIntoView = jest.fn()
  ReactDOM.render(
    <LogPane log="hello\nworld\nfoo" message="world" isExpanded={false} />,
    div
  )
  ReactDOM.unmountComponentAtNode(div)
})

it("renders logs", () => {
  const log = "hello\nworld\nfoo\nbar"
  const tree = renderer
    .create(<LogPane log={log} isExpanded={false} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders logs with leading whitespace and ANSI codes", () => {
  const log = `[32mStarting Tilt (v0.7.10-dev, built 2019-04-10)…[0m
    [Tiltfile] Beginning Tiltfile execution
    [Tiltfile] Running \`"whoami"\`
    Installing Tilt NodeJS dependencies…
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/fe.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/vigoda.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/snack.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/doggos.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/fortune.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/hypothesizer.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/spoonerisms.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/emoji.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/words.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/secrets.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/job.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/sleeper.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/hello_world.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/tick.yaml\""\`
    [Tiltfile] WARNING: This Tiltfile is using k8s resource assembly version 1, which has been deprecated. It will no longer be supported after 2019-04-17. Sorry for the inconvenience! See https://tilt.dev/resource_assembly_migration.html for information on how to migrate.
    [Tiltfile]
    [Tiltfile] Successfully loaded Tiltfile
    [34m  │ [0mApplying via kubectl

    [34m──┤ Building: [0mhello-world[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/1 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 4.138s
    [34m  │ [0mDone in: 4.138s


    [34m──┤ Building: [0mecho-hi[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/1 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 0.556s
    [34m  │ [0mDone in: 0.556s


    [34m──┤ Building: [0mtick[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/1 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 0.450s
    [34m  │ [0mDone in: 0.450s


    [34m──┤ Building: [0mfe[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/3 — [0mBuilding Dockerfile: [docker.io/library/fe]
    Building Dockerfile:
      FROM golang:1.10

      RUN apt update && apt install -y unzip time make

      ENV PROTOC_VERSION 3.5.1

      RUN wget https://github.com/google/protobuf/releases/download/v\${PROTOC_VERSION}/protoc-\${PROTOC_VERSION}-linux-x86_64.zip && \
        unzip protoc-\${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \
        mv protoc/bin/protoc /usr/bin/protoc

      RUN go get github.com/golang/protobuf/protoc-gen-go

      ADD . /go/src/github.com/windmilleng/servantes/fe
      RUN go install github.com/windmilleng/servantes/fe
      ENTRYPOINT /go/bin/fe


    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 24 MB)
    [34m  │ [0mBuilding image
        ╎ [1/6] FROM docker.io/library/golang:1.10
        ╎ [2/6] RUN apt update && apt install -y unzip time make
        ╎ [3/6] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc
        ╎ [4/6] RUN go get github.com/golang/protobuf/protoc-gen-go
        ╎ [5/6] ADD . /go/src/github.com/windmilleng/servantes/fe
        ╎ [6/6] RUN go install github.com/windmilleng/servantes/fe

    [34mSTEP 2/3 — [0mPushing gcr.io/windmill-public-containers/servantes/fe:tilt-2540b7769f4b0e45
        ╎ Skipping push

    [34mSTEP 3/3 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 7.630s
    [34m  │ [0mStep 2 - 0.000s
    [34m  │ [0mStep 3 - 0.249s
    [34m  │ [0mDone in: 7.880s


    [34m──┤ Building: [0mvigoda[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/3 — [0mBuilding Dockerfile: [docker.io/library/vigoda]
    Building Dockerfile:
      FROM golang:1.10

      ADD . /go/src/github.com/windmilleng/servantes/vigoda
      RUN go install github.com/windmilleng/servantes/vigoda

      ENTRYPOINT /go/bin/vigoda

    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 8.7 kB)
    [34m  │ [0mBuilding image
        ╎ [1/3] FROM docker.io/library/golang:1.10
        ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/vigoda
        ╎ [3/3] RUN go install github.com/windmilleng/servantes/vigoda

    [34mSTEP 2/3 — [0mPushing gcr.io/windmill-public-containers/servantes/vigoda:tilt-2d369271c8091f68
        ╎ Skipping push

    [34mSTEP 3/3 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 1.017s
    [34m  │ [0mStep 2 - 0.000s
    [34m  │ [0mStep 3 - 0.168s
    [34m  │ [0mDone in: 1.185s


    [34m──┤ Building: [0msnack[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/3 — [0mBuilding Dockerfile: [docker.io/library/snack]
    Building Dockerfile:
      FROM golang:1.10

      ADD . /go/src/github.com/windmilleng/servantes/snack
      RUN go install github.com/windmilleng/servantes/snack

      ENTRYPOINT /go/bin/snack

    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 9.7 kB)
    [34m  │ [0mBuilding image
        ╎ [1/3] FROM docker.io/library/golang:1.10
    Starting Tilt webpack server…
    fe          ┊ 2019/04/10 15:37:37 Starting Servantes FE on :8080
        ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack
        ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack

    [34mSTEP 2/3 — [0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-731280d503bbbcf5
        ╎ Skipping push

    [34mSTEP 3/3 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 2.878s
    [34m  │ [0mStep 2 - 0.000s
    [34m  │ [0mStep 3 - 0.322s
    [34m  │ [0mDone in: 3.200s


    [34m──┤ Building: [0mdoggos[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/5 — [0mBuilding Dockerfile: [docker.io/library/doggos]
    Building Dockerfile:
      FROM golang:1.10

      ADD . /go/src/github.com/windmilleng/servantes/doggos
      RUN go install github.com/windmilleng/servantes/doggos

      ENTRYPOINT /go/bin/doggos

    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 7.7 kB)
    [34m  │ [0mBuilding image
    vigoda      ┊ 2019/04/10 15:37:39 Starting Vigoda Health Check Service on :8081
        ╎ [1/3] FROM docker.io/library/golang:1.10
        ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/doggos
        ╎ [3/3] RUN go install github.com/windmilleng/servantes/doggos

    [34mSTEP 2/5 — [0mPushing gcr.io/windmill-public-containers/servantes/doggos:tilt-28a4e6fab0991d2f
        ╎ Skipping push

    [34mSTEP 3/5 — [0mBuilding Dockerfile: [docker.io/library/sidecar]
    Building Dockerfile:
      FROM alpine

      ADD loud_sidecar.sh /
      ENTRYPOINT ["/loud_sidecar.sh"]


    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 4.6 kB)
    [34m  │ [0mBuilding image
    vigoda      ┊ 2019/04/10 15:37:41 Server status: All good
        ╎ [1/2] FROM docker.io/library/alpine
        ╎ [2/2] ADD loud_sidecar.sh /

    [34mSTEP 4/5 — [0mPushing gcr.io/windmill-public-containers/servantes/sidecar:tilt-4fb31b5179f3ad01
        ╎ Skipping push

    [34mSTEP 5/5 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 1.629s
    [34m  │ [0mStep 2 - 0.000s
    [34m  │ [0mStep 3 - 2.024s
    [34m  │ [0mStep 4 - 0.000s
    [34m  │ [0mStep 5 - 0.218s
    [34m  │ [0mDone in: 3.871s


    [34m──┤ Building: [0mfortune[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/3 — [0mBuilding Dockerfile: [docker.io/library/fortune]
    Building Dockerfile:
      FROM golang:1.10

      RUN apt update && apt install -y unzip time make

      ENV PROTOC_VERSION 3.5.1

      RUN wget https://github.com/google/protobuf/releases/download/v\${PROTOC_VERSION}/protoc-\${PROTOC_VERSION}-linux-x86_64.zip && \
        unzip protoc-\${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \
        mv protoc/bin/protoc /usr/bin/protoc

      RUN go get github.com/golang/protobuf/protoc-gen-go

      ADD . /go/src/github.com/windmilleng/servantes/fortune
      RUN cd /go/src/github.com/windmilleng/servantes/fortune && make proto
      RUN go install github.com/windmilleng/servantes/fortune

      ENTRYPOINT /go/bin/fortune


    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 16 kB)
    [34m  │ [0mBuilding image
    snack       ┊ 2019/04/10 15:37:41 Starting Snack Service on :8083
    vigoda      ┊ 2019/04/10 15:37:43 Server status: All good
        ╎ [1/7] FROM docker.io/library/golang:1.10
        ╎ [2/7] RUN apt update && apt install -y unzip time make
        ╎ [3/7] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc
        ╎ [4/7] RUN go get github.com/golang/protobuf/protoc-gen-go
        ╎ [5/7] ADD . /go/src/github.com/windmilleng/servantes/fortune
        ╎ [6/7] RUN cd /go/src/github.com/windmilleng/servantes/fortune && make proto
        ╎ [7/7] RUN go install github.com/windmilleng/servantes/fortune

    [34mSTEP 2/3 — [0mPushing gcr.io/windmill-public-containers/servantes/fortune:tilt-7e4331cb0b073360
        ╎ Skipping push

    [34mSTEP 3/3 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 1.634s
    [34m  │ [0mStep 2 - 0.000s
    [34m  │ [0mStep 3 - 0.279s
    [34m  │ [0mDone in: 1.914s


    [34m──┤ Building: [0mhypothesizer[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/3 — [0mBuilding Dockerfile: [docker.io/library/hypothesizer]
    Building Dockerfile:
      FROM python:3.6

      ADD . /app
      RUN cd /app && pip install -r requirements.txt

    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 6.1 kB)
    [34m  │ [0mBuilding image
    vigoda      ┊ 2019/04/10 15:37:45 Server status: All good
        ╎ [1/3] FROM docker.io/library/python:3.6@sha256:fcbf363c285f331894b33f2577e0426182b989c750133a989abaaa4edea324e9
        ╎ [2/3] ADD . /app
        ╎ [3/3] RUN cd /app && pip install -r requirements.txt

    [34mSTEP 2/3 — [0mPushing gcr.io/windmill-public-containers/servantes/hypothesizer:tilt-e2e22b5b98437e29
        ╎ Skipping push

    [34mSTEP 3/3 — [0mDeploying
    [34m  │ [0mParsing Kubernetes config YAML
    [34m  │ [0mApplying via kubectl

    [34m  │ [0mStep 1 - 2.119s
    [34m  │ [0mStep 2 - 0.000s
    [34m  │ [0mStep 3 - 0.517s
    [34m  │ [0mDone in: 2.636s


    [34m──┤ Building: [0mspoonerisms[34m ├──────────────────────────────────────────────[0m
    [34mSTEP 1/3 — [0mBuilding Dockerfile: [docker.io/library/spoonerisms]
    Building Dockerfile:
      FROM node:10

      ADD package.json /app/package.json
      ADD yarn.lock /app/yarn.lock
      RUN cd /app && yarn install

      ADD src /app

      ENTRYPOINT [ "node", "/app/index.js" ]


    [34m  │ [0mTarring context…
        ╎ Created tarball (size: 459 kB)
    [34m  │ [0mBuilding image
    [Tiltfile] Beginning Tiltfile execution
    [Tiltfile] Running \`"whoami"\`
    [Tiltfile]        HI
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/fe.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/vigoda.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/snack.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/doggos.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/fortune.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/hypothesizer.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/spoonerisms.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/emoji.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/words.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/secrets.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/job.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/sleeper.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/hello_world.yaml\""\`
    [Tiltfile] Running \`"m4 -Dvarowner=dan \"deploy/tick.yaml\""\`
    vigoda      ┊ 2019/04/10 15:37:47 Server status: All good
    [Tiltfile] WARNING: This Tiltfile is using k8s resource assembly version 1, which has been deprecated. It will no longer be supported after 2019-04-17. Sorry for the inconvenience! See https://tilt.dev/resource_assembly_migration.html for information on how to migrate.
    [Tiltfile]
    [Tiltfile] Successfully loaded Tiltfile
    doggos      ┊ [doggos] 2019/04/10 15:37:45 Starting Doggos Service on :8083
    doggos      ┊ [sidecar] I'm a loud sidecar! [Wed Apr 10 15:37:46 UTC 2019]
    doggos      ┊ [sidecar] I'm a loud sidecar! [Wed Apr 10 15:37:48 UTC 2019]
    doggos      ┊ [doggos] 2019/04/10 15:37:49 Heartbeat
        ╎ [1/5] FROM docker.io/library/node:10
        ╎ [2/5] ADD package.json /app/package.json
        ╎ [3/5] ADD yarn.lock /app/yarn.lock
        ╎ [4/5] RUN cd /app && yarn install
        ╎ [5/5] ADD src /app`
  const tree = renderer
    .create(<LogPane log={log} isExpanded={false} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})
