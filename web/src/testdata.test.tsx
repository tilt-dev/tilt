import { RouteComponentProps } from "react-router-dom"
import { UnregisterCallback, Href } from "history"
import { Resource, TriggerMode } from "./types"
import { getResourceAlerts } from "./alerts"

type view = {
  resources: Array<Resource>
  featureFlags?: { [featureFlag: string]: boolean }
}

// NOTE(dmiller) 4-02-19 this function is currently unused but I'm going to keep it around.
// I have a feeling that it will come in handy later.
function getMockRouterProps<P>(data: P) {
  var location = {
    hash: "",
    key: "",
    pathname: "",
    search: "",
    state: {},
  }

  var props: RouteComponentProps<P> = {
    match: {
      isExact: true,
      params: data,
      path: "",
      url: "",
    },
    location: location,
    history: {
      length: 2,
      action: "POP",
      location: location,
      push: () => {},
      replace: () => {},
      go: num => {},
      goBack: () => {},
      goForward: () => {},
      block: t => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
      createHref: t => {
        var temp: Href = ""
        return temp
      },
      listen: t => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
    },
    staticContext: {},
  }

  return props
}

function oneResource(): Resource {
  const ts = new Date(Date.now()).toISOString()
  const resource: Resource = {
    name: "vigoda",
    directoriesWatched: ["foo", "bar"],
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: "the build failed!",
        finishTime: ts,
        startTime: ts,
      },
    ],
    pendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    pendingBuildSince: ts,
    pendingBuildReason: 0,
    currentBuild: {
      edits: ["main.go"],
      startTime: ts,
    },
    k8sResourceInfo: {
      podName: "vigoda-pod",
      podCreationTime: ts,
      podStatus: "Running",
      podStatusMessage: "",
      podRestarts: 0,
      podLog: "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
      podUpdateStartTime: ts,
    },
    runtimeStatus: "ok",
    combinedLog: "",
    crashLog: "",
    triggerMode: TriggerMode.TriggerModeAuto,
    hasPendingChanges: false,
    endpoints: [],
    podID: "",
    isTiltfile: false,
    pathsWatched: [],
    alerts: [],
  }
  return resource
}

function oneResourceNoAlerts(): any {
  const ts = Date.now().valueOf()
  const resource = {
    name: "vigoda",
    directoriesWatched: ["foo", "bar"],
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: null,
        finishTime: ts,
        startTime: ts,
      },
    ],
    pendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    pendingBuildSince: ts,
    currentBuild: {},
    k8sResourceInfo: {
      podName: "vigoda-pod",
      podCreationTime: ts,
      podStatus: "Running",
      podRestarts: 0,
      podLog: "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
    },
    endpoints: ["1.2.3.4:8080"],
    runtimeStatus: "ok",
  }
  return resource
}

function oneResourceImagePullBackOff(): any {
  const ts = Date.now().valueOf()
  const resource = {
    name: "vigoda",
    directoriesWatched: ["foo", "bar"],
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: null,
        finishTime: ts,
        startTime: ts,
      },
    ],
    pendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    pendingBuildSince: ts,
    currentBuild: {},
    k8sResourceInfo: {
      podName: "vigoda-pod",
      podCreationTime: ts,
      podStatus: "ImagePullBackOff",
      podRestarts: 0,
      podLog: "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
    },
    endpoints: ["1.2.3.4:8080"],
    runtimeStatus: "ok",
  }
  return resource
}

function oneResourceErrImgPull(): any {
  const ts = Date.now().valueOf()
  const resource = {
    name: "vigoda",
    directoriesWatched: ["foo", "bar"],
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: null,
        finishTime: ts,
        startTime: ts,
      },
    ],
    pendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    pendingBuildSince: ts,
    currentBuild: {},
    k8sResourceInfo: {
      podName: "vigoda-pod",
      podCreationTime: ts,
      podStatus: "ErrImagePull",
      podRestarts: 0,
      podLog: "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
    },
    endpoints: ["1.2.3.4:8080"],
    runtimeStatus: "ok",
  }
  return resource
}

function oneResourceUnrecognizedError(): any {
  const ts = Date.now().valueOf()
  const resource = {
    name: "vigoda",
    directoriesWatched: ["foo", "bar"],
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: null,
        finishTime: ts,
        startTime: ts,
      },
    ],
    pendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    pendingBuildSince: ts,
    currentBuild: {},
    k8sResourceInfo: {
      podName: "vigoda-pod",
      podCreationTime: ts,
      podStatus: "GarbleError",
      podStatusMessage: "Detailed message on GarbleError",
    },
    runtimeStatus: "ok",
  }
  return resource
}

function oneResourceView(): view {
  return { resources: [oneResource()] }
}

function twoResourceView(): view {
  const time = Date.now()
  const ts = new Date(time).toISOString()
  const vigoda = oneResource()

  const snack: Resource = {
    name: "snack",
    directoriesWatched: ["foo", "bar"],
    lastDeployTime: new Date(time - 10000).toISOString(),
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: "the build failed!",
        finishTime: ts,
        startTime: ts,
      },
    ],
    pendingBuildEdits: ["main.go", "cli.go", "snack.go"],
    pendingBuildSince: ts,
    currentBuild: {
      edits: ["main.go"],
      startTime: ts,
    },
    endpoints: ["1.2.3.4:8080"],
    runtimeStatus: "ok",
    triggerMode: TriggerMode.TriggerModeAuto,
    combinedLog: "",
    crashLog: "",
    isTiltfile: false,
    podID: "",
    pathsWatched: [],
    pendingBuildReason: 0,
    k8sResourceInfo: {
      podStatus: "Running",
      podStatusMessage: "",
      podRestarts: 0,
      podCreationTime: "",
      podLog: "",
      podName: "snack",
      podUpdateStartTime: "",
    },
    hasPendingChanges: false,
    alerts: [],
  }
  return { resources: [vigoda, snack] }
}

function allResourcesOK(): any {
  return [
    {
      name: "(Tiltfile)",
      directoriesWatched: null,
      pathsWatched: null,
      lastDeployTime: "2019-04-22T10:59:53.903047-04:00",
      buildHistory: [
        {
          edits: [
            "/Users/dan/go/src/github.com/windmilleng/servantes/Tiltfile",
          ],
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:53.574652-04:00",
          finishTime: "2019-04-22T10:59:53.903047-04:00",
          log:
            'Beginning Tiltfile execution\nRunning `"whoami"`\nRunning `"m4 -Dvarowner=dan \\"deploy/fe.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/vigoda.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/snack.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/doggos.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/fortune.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hypothesizer.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/spoonerisms.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/emoji.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/words.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/secrets.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/job.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/sleeper.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hello_world.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/tick.yaml\\""`\nSuccessfully loaded Tiltfile\n',
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: null,
      runtimeStatus: "ok",
      isTiltfile: true,
      showBuildStatus: false,
      combinedLog:
        'Beginning Tiltfile execution\nRunning `"whoami"`\nRunning `"m4 -Dvarowner=dan \\"deploy/fe.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/vigoda.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/snack.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/doggos.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/fortune.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hypothesizer.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/spoonerisms.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/emoji.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/words.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/secrets.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/job.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/sleeper.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hello_world.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/tick.yaml\\""`\nSuccessfully loaded Tiltfile\n',
    },
    {
      name: "fe",
      directoriesWatched: ["fe"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:01.337285-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:56.489417-04:00",
          finishTime: "2019-04-22T11:00:01.337284-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mfe\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/fe]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  RUN apt update && apt install -y unzip time make\n  \n  ENV PROTOC_VERSION 3.5.1\n  \n  RUN wget https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \\\n    unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \\\n    mv protoc/bin/protoc /usr/bin/protoc\n  \n  RUN go get github.com/golang/protobuf/protoc-gen-go\n  \n  ADD . /go/src/github.com/windmilleng/servantes/fe\n  RUN go install github.com/windmilleng/servantes/fe\n  ENTRYPOINT /go/bin/fe\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 24 MB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/6] FROM docker.io/library/golang:1.10\n    ╎ [2/6] RUN apt update && apt install -y unzip time make\n    ╎ [3/6] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc\n    ╎ [4/6] RUN go get github.com/golang/protobuf/protoc-gen-go\n    ╎ [5/6] ADD . /go/src/github.com/windmilleng/servantes/fe\n    ╎ [6/6] RUN go install github.com/windmilleng/servantes/fe\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/fe:tilt-2540b7769f4b0e45\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 4.628s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.218s\n\u001b[34m  │ \u001b[0mDone in: 4.847s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9000/"],
      k8sResourceInfo: {
        podName: "dan-fe-7cdc8f978f-vp94d",
        podCreationTime: "2019-04-22T11:00:01-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "2019/04/22 15:00:03 Starting Servantes FE on :8080\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mfe\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/fe]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  RUN apt update && apt install -y unzip time make\n  \n  ENV PROTOC_VERSION 3.5.1\n  \n  RUN wget https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \\\n    unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \\\n    mv protoc/bin/protoc /usr/bin/protoc\n  \n  RUN go get github.com/golang/protobuf/protoc-gen-go\n  \n  ADD . /go/src/github.com/windmilleng/servantes/fe\n  RUN go install github.com/windmilleng/servantes/fe\n  ENTRYPOINT /go/bin/fe\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 24 MB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/6] FROM docker.io/library/golang:1.10\n    ╎ [2/6] RUN apt update && apt install -y unzip time make\n    ╎ [3/6] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc\n    ╎ [4/6] RUN go get github.com/golang/protobuf/protoc-gen-go\n    ╎ [5/6] ADD . /go/src/github.com/windmilleng/servantes/fe\n    ╎ [6/6] RUN go install github.com/windmilleng/servantes/fe\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/fe:tilt-2540b7769f4b0e45\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 4.628s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.218s\n\u001b[34m  │ \u001b[0mDone in: 4.847s \n\n2019/04/22 15:00:03 Starting Servantes FE on :8080\n",
    },
    {
      name: "vigoda",
      directoriesWatched: ["vigoda"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:02.810113-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:01.337359-04:00",
          finishTime: "2019-04-22T11:00:02.810112-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mvigoda\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/vigoda]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/vigoda\n  RUN go install github.com/windmilleng/servantes/vigoda\n  \n  ENTRYPOINT /go/bin/vigoda\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 8.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/vigoda\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/vigoda:tilt-2d369271c8091f68\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.283s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.189s\n\u001b[34m  │ \u001b[0mDone in: 1.472s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9001/"],
      k8sResourceInfo: {
        podName: "dan-vigoda-67d79bd8d5-w77q4",
        podCreationTime: "2019-04-22T11:00:02-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog:
          "2019/04/22 15:00:04 Starting Vigoda Health Check Service on :8081\n2019/04/22 15:00:06 Server status: All good\n2019/04/22 15:00:08 Server status: All good\n2019/04/22 15:00:10 Server status: All good\n2019/04/22 15:00:12 Server status: All good\n2019/04/22 15:00:14 Server status: All good\n2019/04/22 15:00:16 Server status: All good\n2019/04/22 15:00:18 Server status: All good\n2019/04/22 15:00:20 Server status: All good\n2019/04/22 15:00:22 Server status: All good\n2019/04/22 15:00:24 Server status: All good\n2019/04/22 15:00:26 Server status: All good\n2019/04/22 15:00:28 Server status: All good\n2019/04/22 15:00:30 Server status: All good\n2019/04/22 15:00:32 Server status: All good\n2019/04/22 15:00:34 Server status: All good\n2019/04/22 15:00:36 Server status: All good\n2019/04/22 15:00:38 Server status: All good\n2019/04/22 15:00:40 Server status: All good\n2019/04/22 15:00:42 Server status: All good\n2019/04/22 15:00:44 Server status: All good\n2019/04/22 15:00:46 Server status: All good\n2019/04/22 15:00:48 Server status: All good\n2019/04/22 15:00:50 Server status: All good\n2019/04/22 15:00:52 Server status: All good\n2019/04/22 15:00:54 Server status: All good\n2019/04/22 15:00:56 Server status: All good\n2019/04/22 15:00:58 Server status: All good\n2019/04/22 15:01:00 Server status: All good\n2019/04/22 15:01:02 Server status: All good\n2019/04/22 15:01:04 Server status: All good\n2019/04/22 15:01:06 Server status: All good\n2019/04/22 15:01:08 Server status: All good\n2019/04/22 15:01:10 Server status: All good\n2019/04/22 15:01:12 Server status: All good\n2019/04/22 15:01:14 Server status: All good\n2019/04/22 15:01:16 Server status: All good\n2019/04/22 15:01:18 Server status: All good\n2019/04/22 15:01:20 Server status: All good\n2019/04/22 15:01:22 Server status: All good\n2019/04/22 15:01:24 Server status: All good\n2019/04/22 15:01:26 Server status: All good\n2019/04/22 15:01:28 Server status: All good\n2019/04/22 15:01:30 Server status: All good\n2019/04/22 15:01:32 Server status: All good\n2019/04/22 15:01:34 Server status: All good\n2019/04/22 15:01:36 Server status: All good\n2019/04/22 15:01:38 Server status: All good\n2019/04/22 15:01:40 Server status: All good\n2019/04/22 15:01:42 Server status: All good\n2019/04/22 15:01:44 Server status: All good\n2019/04/22 15:01:46 Server status: All good\n2019/04/22 15:01:48 Server status: All good\n2019/04/22 15:01:50 Server status: All good\n2019/04/22 15:01:52 Server status: All good\n2019/04/22 15:01:54 Server status: All good\n2019/04/22 15:01:56 Server status: All good\n2019/04/22 15:01:58 Server status: All good\n2019/04/22 15:02:00 Server status: All good\n2019/04/22 15:02:02 Server status: All good\n2019/04/22 15:02:04 Server status: All good\n2019/04/22 15:02:06 Server status: All good\n2019/04/22 15:02:08 Server status: All good\n2019/04/22 15:02:10 Server status: All good\n2019/04/22 15:02:12 Server status: All good\n2019/04/22 15:02:14 Server status: All good\n2019/04/22 15:02:16 Server status: All good\n2019/04/22 15:02:18 Server status: All good\n2019/04/22 15:02:20 Server status: All good\n2019/04/22 15:02:22 Server status: All good\n2019/04/22 15:02:24 Server status: All good\n2019/04/22 15:02:26 Server status: All good\n2019/04/22 15:02:28 Server status: All good\n2019/04/22 15:02:30 Server status: All good\n2019/04/22 15:02:32 Server status: All good\n2019/04/22 15:02:34 Server status: All good\n2019/04/22 15:02:36 Server status: All good\n2019/04/22 15:02:38 Server status: All good\n2019/04/22 15:02:40 Server status: All good\n2019/04/22 15:02:42 Server status: All good\n2019/04/22 15:02:44 Server status: All good\n2019/04/22 15:02:46 Server status: All good\n2019/04/22 15:02:48 Server status: All good\n2019/04/22 15:02:50 Server status: All good\n2019/04/22 15:02:52 Server status: All good\n2019/04/22 15:02:54 Server status: All good\n2019/04/22 15:02:56 Server status: All good\n2019/04/22 15:02:58 Server status: All good\n2019/04/22 15:03:00 Server status: All good\n2019/04/22 15:03:02 Server status: All good\n2019/04/22 15:03:04 Server status: All good\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mvigoda\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/vigoda]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/vigoda\n  RUN go install github.com/windmilleng/servantes/vigoda\n  \n  ENTRYPOINT /go/bin/vigoda\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 8.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/vigoda\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/vigoda:tilt-2d369271c8091f68\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.283s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.189s\n\u001b[34m  │ \u001b[0mDone in: 1.472s \n\n2019/04/22 15:00:04 Starting Vigoda Health Check Service on :8081\n2019/04/22 15:00:06 Server status: All good\n2019/04/22 15:00:08 Server status: All good\n2019/04/22 15:00:10 Server status: All good\n2019/04/22 15:00:12 Server status: All good\n2019/04/22 15:00:14 Server status: All good\n2019/04/22 15:00:16 Server status: All good\n2019/04/22 15:00:18 Server status: All good\n2019/04/22 15:00:20 Server status: All good\n2019/04/22 15:00:22 Server status: All good\n2019/04/22 15:00:24 Server status: All good\n2019/04/22 15:00:26 Server status: All good\n2019/04/22 15:00:28 Server status: All good\n2019/04/22 15:00:30 Server status: All good\n2019/04/22 15:00:32 Server status: All good\n2019/04/22 15:00:34 Server status: All good\n2019/04/22 15:00:36 Server status: All good\n2019/04/22 15:00:38 Server status: All good\n2019/04/22 15:00:40 Server status: All good\n2019/04/22 15:00:42 Server status: All good\n2019/04/22 15:00:44 Server status: All good\n2019/04/22 15:00:46 Server status: All good\n2019/04/22 15:00:48 Server status: All good\n2019/04/22 15:00:50 Server status: All good\n2019/04/22 15:00:52 Server status: All good\n2019/04/22 15:00:54 Server status: All good\n2019/04/22 15:00:56 Server status: All good\n2019/04/22 15:00:58 Server status: All good\n2019/04/22 15:01:00 Server status: All good\n2019/04/22 15:01:02 Server status: All good\n2019/04/22 15:01:04 Server status: All good\n2019/04/22 15:01:06 Server status: All good\n2019/04/22 15:01:08 Server status: All good\n2019/04/22 15:01:10 Server status: All good\n2019/04/22 15:01:12 Server status: All good\n2019/04/22 15:01:14 Server status: All good\n2019/04/22 15:01:16 Server status: All good\n2019/04/22 15:01:18 Server status: All good\n2019/04/22 15:01:20 Server status: All good\n2019/04/22 15:01:22 Server status: All good\n2019/04/22 15:01:24 Server status: All good\n2019/04/22 15:01:26 Server status: All good\n2019/04/22 15:01:28 Server status: All good\n2019/04/22 15:01:30 Server status: All good\n2019/04/22 15:01:32 Server status: All good\n2019/04/22 15:01:34 Server status: All good\n2019/04/22 15:01:36 Server status: All good\n2019/04/22 15:01:38 Server status: All good\n2019/04/22 15:01:40 Server status: All good\n2019/04/22 15:01:42 Server status: All good\n2019/04/22 15:01:44 Server status: All good\n2019/04/22 15:01:46 Server status: All good\n2019/04/22 15:01:48 Server status: All good\n2019/04/22 15:01:50 Server status: All good\n2019/04/22 15:01:52 Server status: All good\n2019/04/22 15:01:54 Server status: All good\n2019/04/22 15:01:56 Server status: All good\n2019/04/22 15:01:58 Server status: All good\n2019/04/22 15:02:00 Server status: All good\n2019/04/22 15:02:02 Server status: All good\n2019/04/22 15:02:04 Server status: All good\n2019/04/22 15:02:06 Server status: All good\n2019/04/22 15:02:08 Server status: All good\n2019/04/22 15:02:10 Server status: All good\n2019/04/22 15:02:12 Server status: All good\n2019/04/22 15:02:14 Server status: All good\n2019/04/22 15:02:16 Server status: All good\n2019/04/22 15:02:18 Server status: All good\n2019/04/22 15:02:20 Server status: All good\n2019/04/22 15:02:22 Server status: All good\n2019/04/22 15:02:24 Server status: All good\n2019/04/22 15:02:26 Server status: All good\n2019/04/22 15:02:28 Server status: All good\n2019/04/22 15:02:30 Server status: All good\n2019/04/22 15:02:32 Server status: All good\n2019/04/22 15:02:34 Server status: All good\n2019/04/22 15:02:36 Server status: All good\n2019/04/22 15:02:38 Server status: All good\n2019/04/22 15:02:40 Server status: All good\n2019/04/22 15:02:42 Server status: All good\n2019/04/22 15:02:44 Server status: All good\n2019/04/22 15:02:46 Server status: All good\n2019/04/22 15:02:48 Server status: All good\n2019/04/22 15:02:50 Server status: All good\n2019/04/22 15:02:52 Server status: All good\n2019/04/22 15:02:54 Server status: All good\n2019/04/22 15:02:56 Server status: All good\n2019/04/22 15:02:58 Server status: All good\n2019/04/22 15:03:00 Server status: All good\n2019/04/22 15:03:02 Server status: All good\n2019/04/22 15:03:04 Server status: All good\n",
    },
    {
      name: "snack",
      directoriesWatched: ["snack"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:04.242586-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:02.810268-04:00",
          finishTime: "2019-04-22T11:00:04.242583-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-13631d4ed09f1a05\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.241s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.190s\n\u001b[34m  │ \u001b[0mDone in: 1.431s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9002/"],
      k8sResourceInfo: {
        podName: "dan-snack-f885fb46f-d5z2t",
        podCreationTime: "2019-04-22T11:00:04-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "2019/04/22 15:00:06 Starting Snack Service on :8083\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-13631d4ed09f1a05\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.241s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.190s\n\u001b[34m  │ \u001b[0mDone in: 1.431s \n\n2019/04/22 15:00:06 Starting Snack Service on :8083\n",
    },
    {
      name: "doggos",
      directoriesWatched: ["doggos", "sidecar"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:07.804953-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:04.242664-04:00",
          finishTime: "2019-04-22T11:00:07.804952-04:00",
          log:
            '\n\u001b[34m──┤ Building: \u001b[0mdoggos\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/5 — \u001b[0mBuilding Dockerfile: [docker.io/library/doggos]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/doggos\n  RUN go install github.com/windmilleng/servantes/doggos\n  \n  ENTRYPOINT /go/bin/doggos\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 7.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/doggos\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/doggos\n\n\u001b[34mSTEP 2/5 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/doggos:tilt-28a4e6fab0991d2f\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/5 — \u001b[0mBuilding Dockerfile: [docker.io/library/sidecar]\nBuilding Dockerfile:\n  FROM alpine\n  \n  ADD loud_sidecar.sh /\n  ENTRYPOINT ["/loud_sidecar.sh"]\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 4.6 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/2] FROM docker.io/library/alpine\n    ╎ [2/2] ADD loud_sidecar.sh /\n\n\u001b[34mSTEP 4/5 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/sidecar:tilt-4fb31b5179f3ad01\n    ╎ Skipping push\n\n\u001b[34mSTEP 5/5 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.856s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 1.483s\n\u001b[34m  │ \u001b[0mStep 4 - 0.000s\n\u001b[34m  │ \u001b[0mStep 5 - 0.222s\n\u001b[34m  │ \u001b[0mDone in: 3.561s \n\n',
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9003/"],
      k8sResourceInfo: {
        podName: "dan-doggos-596cc68bd9-w87f8",
        podCreationTime: "2019-04-22T11:00:07-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog:
          "[doggos] 2019/04/22 15:00:10 Starting Doggos Service on :8083\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:10 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:12 UTC 2019]\n[doggos] 2019/04/22 15:00:13 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:14 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:16 UTC 2019]\n[doggos] 2019/04/22 15:00:16 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:18 UTC 2019]\n[doggos] 2019/04/22 15:00:19 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:20 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:22 UTC 2019]\n[doggos] 2019/04/22 15:00:22 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:24 UTC 2019]\n[doggos] 2019/04/22 15:00:26 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:26 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:28 UTC 2019]\n[doggos] 2019/04/22 15:00:29 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:30 UTC 2019]\n[doggos] 2019/04/22 15:00:32 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:32 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:34 UTC 2019]\n[doggos] 2019/04/22 15:00:35 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:36 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:38 UTC 2019]\n[doggos] 2019/04/22 15:00:38 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:40 UTC 2019]\n[doggos] 2019/04/22 15:00:42 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:42 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:44 UTC 2019]\n[doggos] 2019/04/22 15:00:45 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:46 UTC 2019]\n[doggos] 2019/04/22 15:00:48 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:48 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:50 UTC 2019]\n[doggos] 2019/04/22 15:00:51 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:52 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:54 UTC 2019]\n[doggos] 2019/04/22 15:00:54 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:56 UTC 2019]\n[doggos] 2019/04/22 15:00:58 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:58 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:00 UTC 2019]\n[doggos] 2019/04/22 15:01:01 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:02 UTC 2019]\n[doggos] 2019/04/22 15:01:04 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:04 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:06 UTC 2019]\n[doggos] 2019/04/22 15:01:07 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:08 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:10 UTC 2019]\n[doggos] 2019/04/22 15:01:10 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:12 UTC 2019]\n[doggos] 2019/04/22 15:01:14 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:14 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:16 UTC 2019]\n[doggos] 2019/04/22 15:01:17 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:18 UTC 2019]\n[doggos] 2019/04/22 15:01:20 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:20 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:22 UTC 2019]\n[doggos] 2019/04/22 15:01:23 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:24 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:26 UTC 2019]\n[doggos] 2019/04/22 15:01:26 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:28 UTC 2019]\n[doggos] 2019/04/22 15:01:30 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:30 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:32 UTC 2019]\n[doggos] 2019/04/22 15:01:33 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:34 UTC 2019]\n[doggos] 2019/04/22 15:01:36 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:36 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:38 UTC 2019]\n[doggos] 2019/04/22 15:01:39 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:40 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:42 UTC 2019]\n[doggos] 2019/04/22 15:01:42 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:44 UTC 2019]\n[doggos] 2019/04/22 15:01:46 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:46 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:48 UTC 2019]\n[doggos] 2019/04/22 15:01:49 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:50 UTC 2019]\n[doggos] 2019/04/22 15:01:52 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:52 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:54 UTC 2019]\n[doggos] 2019/04/22 15:01:55 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:56 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:58 UTC 2019]\n[doggos] 2019/04/22 15:01:58 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:00 UTC 2019]\n[doggos] 2019/04/22 15:02:02 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:02 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:04 UTC 2019]\n[doggos] 2019/04/22 15:02:05 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:06 UTC 2019]\n[doggos] 2019/04/22 15:02:08 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:08 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:10 UTC 2019]\n[doggos] 2019/04/22 15:02:11 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:12 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:14 UTC 2019]\n[doggos] 2019/04/22 15:02:14 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:16 UTC 2019]\n[doggos] 2019/04/22 15:02:18 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:18 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:20 UTC 2019]\n[doggos] 2019/04/22 15:02:21 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:22 UTC 2019]\n[doggos] 2019/04/22 15:02:24 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:24 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:26 UTC 2019]\n[doggos] 2019/04/22 15:02:27 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:28 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:30 UTC 2019]\n[doggos] 2019/04/22 15:02:30 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:32 UTC 2019]\n[doggos] 2019/04/22 15:02:34 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:34 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:36 UTC 2019]\n[doggos] 2019/04/22 15:02:37 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:38 UTC 2019]\n[doggos] 2019/04/22 15:02:40 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:40 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:42 UTC 2019]\n[doggos] 2019/04/22 15:02:43 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:44 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:46 UTC 2019]\n[doggos] 2019/04/22 15:02:46 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:48 UTC 2019]\n[doggos] 2019/04/22 15:02:50 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:50 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:52 UTC 2019]\n[doggos] 2019/04/22 15:02:53 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:54 UTC 2019]\n[doggos] 2019/04/22 15:02:56 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:56 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:58 UTC 2019]\n[doggos] 2019/04/22 15:02:59 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:03:00 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:03:02 UTC 2019]\n[doggos] 2019/04/22 15:03:02 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:03:04 UTC 2019]\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mdoggos\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/5 — \u001b[0mBuilding Dockerfile: [docker.io/library/doggos]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/doggos\n  RUN go install github.com/windmilleng/servantes/doggos\n  \n  ENTRYPOINT /go/bin/doggos\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 7.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/doggos\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/doggos\n\n\u001b[34mSTEP 2/5 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/doggos:tilt-28a4e6fab0991d2f\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/5 — \u001b[0mBuilding Dockerfile: [docker.io/library/sidecar]\nBuilding Dockerfile:\n  FROM alpine\n  \n  ADD loud_sidecar.sh /\n  ENTRYPOINT [\"/loud_sidecar.sh\"]\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 4.6 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/2] FROM docker.io/library/alpine\n    ╎ [2/2] ADD loud_sidecar.sh /\n\n\u001b[34mSTEP 4/5 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/sidecar:tilt-4fb31b5179f3ad01\n    ╎ Skipping push\n\n\u001b[34mSTEP 5/5 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.856s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 1.483s\n\u001b[34m  │ \u001b[0mStep 4 - 0.000s\n\u001b[34m  │ \u001b[0mStep 5 - 0.222s\n\u001b[34m  │ \u001b[0mDone in: 3.561s \n\n[doggos] 2019/04/22 15:00:10 Starting Doggos Service on :8083\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:10 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:12 UTC 2019]\n[doggos] 2019/04/22 15:00:13 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:14 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:16 UTC 2019]\n[doggos] 2019/04/22 15:00:16 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:18 UTC 2019]\n[doggos] 2019/04/22 15:00:19 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:20 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:22 UTC 2019]\n[doggos] 2019/04/22 15:00:22 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:24 UTC 2019]\n[doggos] 2019/04/22 15:00:26 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:26 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:28 UTC 2019]\n[doggos] 2019/04/22 15:00:29 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:30 UTC 2019]\n[doggos] 2019/04/22 15:00:32 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:32 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:34 UTC 2019]\n[doggos] 2019/04/22 15:00:35 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:36 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:38 UTC 2019]\n[doggos] 2019/04/22 15:00:38 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:40 UTC 2019]\n[doggos] 2019/04/22 15:00:42 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:42 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:44 UTC 2019]\n[doggos] 2019/04/22 15:00:45 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:46 UTC 2019]\n[doggos] 2019/04/22 15:00:48 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:48 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:50 UTC 2019]\n[doggos] 2019/04/22 15:00:51 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:52 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:54 UTC 2019]\n[doggos] 2019/04/22 15:00:54 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:56 UTC 2019]\n[doggos] 2019/04/22 15:00:58 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:00:58 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:00 UTC 2019]\n[doggos] 2019/04/22 15:01:01 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:02 UTC 2019]\n[doggos] 2019/04/22 15:01:04 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:04 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:06 UTC 2019]\n[doggos] 2019/04/22 15:01:07 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:08 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:10 UTC 2019]\n[doggos] 2019/04/22 15:01:10 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:12 UTC 2019]\n[doggos] 2019/04/22 15:01:14 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:14 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:16 UTC 2019]\n[doggos] 2019/04/22 15:01:17 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:18 UTC 2019]\n[doggos] 2019/04/22 15:01:20 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:20 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:22 UTC 2019]\n[doggos] 2019/04/22 15:01:23 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:24 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:26 UTC 2019]\n[doggos] 2019/04/22 15:01:26 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:28 UTC 2019]\n[doggos] 2019/04/22 15:01:30 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:30 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:32 UTC 2019]\n[doggos] 2019/04/22 15:01:33 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:34 UTC 2019]\n[doggos] 2019/04/22 15:01:36 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:36 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:38 UTC 2019]\n[doggos] 2019/04/22 15:01:39 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:40 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:42 UTC 2019]\n[doggos] 2019/04/22 15:01:42 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:44 UTC 2019]\n[doggos] 2019/04/22 15:01:46 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:46 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:48 UTC 2019]\n[doggos] 2019/04/22 15:01:49 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:50 UTC 2019]\n[doggos] 2019/04/22 15:01:52 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:52 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:54 UTC 2019]\n[doggos] 2019/04/22 15:01:55 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:56 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:01:58 UTC 2019]\n[doggos] 2019/04/22 15:01:58 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:00 UTC 2019]\n[doggos] 2019/04/22 15:02:02 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:02 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:04 UTC 2019]\n[doggos] 2019/04/22 15:02:05 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:06 UTC 2019]\n[doggos] 2019/04/22 15:02:08 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:08 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:10 UTC 2019]\n[doggos] 2019/04/22 15:02:11 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:12 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:14 UTC 2019]\n[doggos] 2019/04/22 15:02:14 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:16 UTC 2019]\n[doggos] 2019/04/22 15:02:18 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:18 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:20 UTC 2019]\n[doggos] 2019/04/22 15:02:21 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:22 UTC 2019]\n[doggos] 2019/04/22 15:02:24 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:24 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:26 UTC 2019]\n[doggos] 2019/04/22 15:02:27 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:28 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:30 UTC 2019]\n[doggos] 2019/04/22 15:02:30 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:32 UTC 2019]\n[doggos] 2019/04/22 15:02:34 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:34 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:36 UTC 2019]\n[doggos] 2019/04/22 15:02:37 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:38 UTC 2019]\n[doggos] 2019/04/22 15:02:40 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:40 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:42 UTC 2019]\n[doggos] 2019/04/22 15:02:43 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:44 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:46 UTC 2019]\n[doggos] 2019/04/22 15:02:46 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:48 UTC 2019]\n[doggos] 2019/04/22 15:02:50 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:50 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:52 UTC 2019]\n[doggos] 2019/04/22 15:02:53 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:54 UTC 2019]\n[doggos] 2019/04/22 15:02:56 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:56 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:02:58 UTC 2019]\n[doggos] 2019/04/22 15:02:59 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:03:00 UTC 2019]\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:03:02 UTC 2019]\n[doggos] 2019/04/22 15:03:02 Heartbeat\n[sidecar] I'm a loud sidecar! [Mon Apr 22 15:03:04 UTC 2019]\n",
    },
    {
      name: "fortune",
      directoriesWatched: ["fortune"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:09.205571-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:07.805076-04:00",
          finishTime: "2019-04-22T11:00:09.205568-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mfortune\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/fortune]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  RUN apt update && apt install -y unzip time make\n  \n  ENV PROTOC_VERSION 3.5.1\n  \n  RUN wget https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \\\n    unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \\\n    mv protoc/bin/protoc /usr/bin/protoc\n  \n  RUN go get github.com/golang/protobuf/protoc-gen-go\n  \n  ADD . /go/src/github.com/windmilleng/servantes/fortune\n  RUN cd /go/src/github.com/windmilleng/servantes/fortune && make proto\n  RUN go install github.com/windmilleng/servantes/fortune\n  \n  ENTRYPOINT /go/bin/fortune\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 16 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/7] FROM docker.io/library/golang:1.10\n    ╎ [2/7] RUN apt update && apt install -y unzip time make\n    ╎ [3/7] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc\n    ╎ [4/7] RUN go get github.com/golang/protobuf/protoc-gen-go\n    ╎ [5/7] ADD . /go/src/github.com/windmilleng/servantes/fortune\n    ╎ [6/7] RUN cd /go/src/github.com/windmilleng/servantes/fortune && make proto\n    ╎ [7/7] RUN go install github.com/windmilleng/servantes/fortune\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/fortune:tilt-7e4331cb0b073360\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.226s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.173s\n\u001b[34m  │ \u001b[0mDone in: 1.399s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9004/"],
      k8sResourceInfo: {
        podName: "dan-fortune-76bcccc6bb-lzzx4",
        podCreationTime: "2019-04-22T11:00:09-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "2019/04/22 15:00:11 Starting Fortune Service on :8082\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mfortune\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/fortune]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  RUN apt update && apt install -y unzip time make\n  \n  ENV PROTOC_VERSION 3.5.1\n  \n  RUN wget https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \\\n    unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \\\n    mv protoc/bin/protoc /usr/bin/protoc\n  \n  RUN go get github.com/golang/protobuf/protoc-gen-go\n  \n  ADD . /go/src/github.com/windmilleng/servantes/fortune\n  RUN cd /go/src/github.com/windmilleng/servantes/fortune && make proto\n  RUN go install github.com/windmilleng/servantes/fortune\n  \n  ENTRYPOINT /go/bin/fortune\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 16 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/7] FROM docker.io/library/golang:1.10\n    ╎ [2/7] RUN apt update && apt install -y unzip time make\n    ╎ [3/7] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc\n    ╎ [4/7] RUN go get github.com/golang/protobuf/protoc-gen-go\n    ╎ [5/7] ADD . /go/src/github.com/windmilleng/servantes/fortune\n    ╎ [6/7] RUN cd /go/src/github.com/windmilleng/servantes/fortune && make proto\n    ╎ [7/7] RUN go install github.com/windmilleng/servantes/fortune\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/fortune:tilt-7e4331cb0b073360\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.226s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.173s\n\u001b[34m  │ \u001b[0mDone in: 1.399s \n\n2019/04/22 15:00:11 Starting Fortune Service on :8082\n",
    },
    {
      name: "hypothesizer",
      directoriesWatched: ["hypothesizer"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:11.203884-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:09.205679-04:00",
          finishTime: "2019-04-22T11:00:11.203881-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mhypothesizer\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/hypothesizer]\nBuilding Dockerfile:\n  FROM python:3.6\n  \n  ADD . /app\n  RUN cd /app && pip install -r requirements.txt\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 6.1 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/python:3.6@sha256:976cd81b859b13ef6c1366517f14bd13754f535fdb3eb41c252214fdd3245dde\n    ╎ [2/3] ADD . /app\n    ╎ [3/3] RUN cd /app && pip install -r requirements.txt\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/hypothesizer:tilt-e2e22b5b98437e29\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.782s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.215s\n\u001b[34m  │ \u001b[0mDone in: 1.997s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9005/"],
      k8sResourceInfo: {
        podName: "dan-hypothesizer-84b486bbfd-qrqd6",
        podCreationTime: "2019-04-22T11:00:11-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog:
          'hello world people!\n * Serving Flask app "app" (lazy loading)\n * Environment: production\n   WARNING: Do not use the development server in a production environment.\n   Use a production WSGI server instead.\n * Debug mode: on\n * Running on http://0.0.0.0:5000/ (Press CTRL+C to quit)\n * Restarting with stat\n * Debugger is active!\n * Debugger PIN: 118-802-155\n',
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        '\n\u001b[34m──┤ Building: \u001b[0mhypothesizer\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/hypothesizer]\nBuilding Dockerfile:\n  FROM python:3.6\n  \n  ADD . /app\n  RUN cd /app && pip install -r requirements.txt\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 6.1 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/python:3.6@sha256:976cd81b859b13ef6c1366517f14bd13754f535fdb3eb41c252214fdd3245dde\n    ╎ [2/3] ADD . /app\n    ╎ [3/3] RUN cd /app && pip install -r requirements.txt\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/hypothesizer:tilt-e2e22b5b98437e29\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.782s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.215s\n\u001b[34m  │ \u001b[0mDone in: 1.997s \n\nhello world people!\n * Serving Flask app "app" (lazy loading)\n * Environment: production\n   WARNING: Do not use the development server in a production environment.\n   Use a production WSGI server instead.\n * Debug mode: on\n * Running on http://0.0.0.0:5000/ (Press CTRL+C to quit)\n * Restarting with stat\n * Debugger is active!\n * Debugger PIN: 118-802-155\n',
    },
    {
      name: "spoonerisms",
      directoriesWatched: ["spoonerisms"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:12.42127-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:11.20396-04:00",
          finishTime: "2019-04-22T11:00:12.421269-04:00",
          log:
            '\n\u001b[34m──┤ Building: \u001b[0mspoonerisms\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/spoonerisms]\nBuilding Dockerfile:\n  FROM node:10\n  \n  ADD package.json /app/package.json\n  ADD yarn.lock /app/yarn.lock\n  RUN cd /app && yarn install\n  \n  ADD src /app\n  \n  ENTRYPOINT [ "node", "/app/index.js" ]\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 459 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/5] FROM docker.io/library/node:10\n    ╎ [2/5] ADD package.json /app/package.json\n    ╎ [3/5] ADD yarn.lock /app/yarn.lock\n    ╎ [4/5] RUN cd /app && yarn install\n    ╎ [5/5] ADD src /app\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/spoonerisms:tilt-b4b16ad1302bfca2\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.015s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.201s\n\u001b[34m  │ \u001b[0mDone in: 1.216s \n\n',
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9006/"],
      k8sResourceInfo: {
        podName: "dan-spoonerisms-bb9577494-lq5w9",
        podCreationTime: "2019-04-22T11:00:12-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "Server running at http://127.0.0.1:5000/\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        '\n\u001b[34m──┤ Building: \u001b[0mspoonerisms\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/spoonerisms]\nBuilding Dockerfile:\n  FROM node:10\n  \n  ADD package.json /app/package.json\n  ADD yarn.lock /app/yarn.lock\n  RUN cd /app && yarn install\n  \n  ADD src /app\n  \n  ENTRYPOINT [ "node", "/app/index.js" ]\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 459 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/5] FROM docker.io/library/node:10\n    ╎ [2/5] ADD package.json /app/package.json\n    ╎ [3/5] ADD yarn.lock /app/yarn.lock\n    ╎ [4/5] RUN cd /app && yarn install\n    ╎ [5/5] ADD src /app\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/spoonerisms:tilt-b4b16ad1302bfca2\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.015s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.201s\n\u001b[34m  │ \u001b[0mDone in: 1.216s \n\nServer running at http://127.0.0.1:5000/\n',
    },
    {
      name: "emoji",
      directoriesWatched: ["emoji"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:13.940312-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:12.421344-04:00",
          finishTime: "2019-04-22T11:00:13.94031-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0memoji\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/emoji]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/emoji\n  RUN go install github.com/windmilleng/servantes/emoji\n  \n  ENTRYPOINT /go/bin/emoji\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 33 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/emoji\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/emoji\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/emoji:tilt-a6e00fe8bd11bb7a\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.269s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.248s\n\u001b[34m  │ \u001b[0mDone in: 1.518s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9007/"],
      k8sResourceInfo: {
        podName: "dan-emoji-6765c9676c-7d655",
        podCreationTime: "2019-04-22T11:00:13-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "2019/04/22 15:00:16 Starting Emoji Service on :8081\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0memoji\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/emoji]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/emoji\n  RUN go install github.com/windmilleng/servantes/emoji\n  \n  ENTRYPOINT /go/bin/emoji\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 33 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/emoji\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/emoji\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/emoji:tilt-a6e00fe8bd11bb7a\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.269s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.248s\n\u001b[34m  │ \u001b[0mDone in: 1.518s \n\n2019/04/22 15:00:16 Starting Emoji Service on :8081\n",
    },
    {
      name: "words",
      directoriesWatched: ["words"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:15.745111-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:13.940432-04:00",
          finishTime: "2019-04-22T11:00:15.745108-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mwords\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/words]\nBuilding Dockerfile:\n  FROM python:3.6\n  \n  ADD . /app\n  RUN cd /app && pip install -r requirements.txt\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 6.1 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/python:3.6@sha256:976cd81b859b13ef6c1366517f14bd13754f535fdb3eb41c252214fdd3245dde\n    ╎ [2/3] ADD . /app\n    ╎ [3/3] RUN cd /app && pip install -r requirements.txt\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/words:tilt-edf98dac53c4f1bc\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.588s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.216s\n\u001b[34m  │ \u001b[0mDone in: 1.804s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9008/"],
      k8sResourceInfo: {
        podName: "dan-words-5bfdf8db84-vdqz4",
        podCreationTime: "2019-04-22T11:00:15-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog:
          'don\'t have wordnet, downloading\n[nltk_data] Downloading package wordnet to /root/nltk_data...\n[nltk_data]   Unzipping corpora/wordnet.zip.\n * Serving Flask app "app" (lazy loading)\n * Environment: production\n   WARNING: Do not use the development server in a production environment.\n   Use a production WSGI server instead.\n * Debug mode: on\n * Running on http://0.0.0.0:5000/ (Press CTRL+C to quit)\n * Restarting with stat\n * Debugger is active!\n * Debugger PIN: 176-349-149\n',
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        '\n\u001b[34m──┤ Building: \u001b[0mwords\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/words]\nBuilding Dockerfile:\n  FROM python:3.6\n  \n  ADD . /app\n  RUN cd /app && pip install -r requirements.txt\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 6.1 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/python:3.6@sha256:976cd81b859b13ef6c1366517f14bd13754f535fdb3eb41c252214fdd3245dde\n    ╎ [2/3] ADD . /app\n    ╎ [3/3] RUN cd /app && pip install -r requirements.txt\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/words:tilt-edf98dac53c4f1bc\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.588s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.216s\n\u001b[34m  │ \u001b[0mDone in: 1.804s \n\ndon\'t have wordnet, downloading\n[nltk_data] Downloading package wordnet to /root/nltk_data...\n[nltk_data]   Unzipping corpora/wordnet.zip.\n * Serving Flask app "app" (lazy loading)\n * Environment: production\n   WARNING: Do not use the development server in a production environment.\n   Use a production WSGI server instead.\n * Debug mode: on\n * Running on http://0.0.0.0:5000/ (Press CTRL+C to quit)\n * Restarting with stat\n * Debugger is active!\n * Debugger PIN: 176-349-149\n',
    },
    {
      name: "secrets",
      directoriesWatched: ["secrets"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:17.035014-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:15.745238-04:00",
          finishTime: "2019-04-22T11:00:17.035013-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0msecrets\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/secrets]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/secrets\n  RUN go install github.com/windmilleng/servantes/secrets\n  \n  ENTRYPOINT /go/bin/secrets\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 7.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/secrets\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/secrets\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/secrets:tilt-7f9376a1d8c74bb3\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.103s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.186s\n\u001b[34m  │ \u001b[0mDone in: 1.289s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9009/"],
      k8sResourceInfo: {
        podName: "dan-secrets-79c8bb5c79-7hwp6",
        podCreationTime: "2019-04-22T11:00:17-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0msecrets\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/secrets]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/secrets\n  RUN go install github.com/windmilleng/servantes/secrets\n  \n  ENTRYPOINT /go/bin/secrets\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 7.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/secrets\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/secrets\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/secrets:tilt-7f9376a1d8c74bb3\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.103s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.186s\n\u001b[34m  │ \u001b[0mDone in: 1.289s \n\n",
    },
    {
      name: "echo-hi",
      directoriesWatched: [],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T10:59:56.010299-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:53.906775-04:00",
          finishTime: "2019-04-22T10:59:56.010298-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mecho-hi\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 2.102s\n\u001b[34m  │ \u001b[0mDone in: 2.102s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: null,
      k8sResourceInfo: {
        podName: "echo-hi-92tww",
        podCreationTime: "2019-04-22T10:59:56-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Completed",
        podRestarts: 0,
        podLog: "",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: false,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mecho-hi\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 2.102s\n\u001b[34m  │ \u001b[0mDone in: 2.102s \n\n",
    },
    {
      name: "sleep",
      directoriesWatched: ["sleeper"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:18.621166-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:17.035107-04:00",
          finishTime: "2019-04-22T11:00:18.621163-04:00",
          log:
            '\n\u001b[34m──┤ Building: \u001b[0msleep\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/sleep]\nBuilding Dockerfile:\n  FROM node:10\n  \n  ADD . /\n  \n  ENTRYPOINT [ "node", "index.js" ]\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 4.6 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/2] FROM docker.io/library/node:10\n    ╎ [2/2] ADD . /\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/sleep:tilt-7175871cc674cce5\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.343s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.241s\n\u001b[34m  │ \u001b[0mDone in: 1.585s \n\n',
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: null,
      k8sResourceInfo: {
        podName: "sleep",
        podCreationTime: "2019-04-22T11:00:18-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Completed",
        podRestarts: 0,
        podLog: "Taking a break...\nTen seconds later\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        '\n\u001b[34m──┤ Building: \u001b[0msleep\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/sleep]\nBuilding Dockerfile:\n  FROM node:10\n  \n  ADD . /\n  \n  ENTRYPOINT [ "node", "index.js" ]\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 4.6 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/2] FROM docker.io/library/node:10\n    ╎ [2/2] ADD . /\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/sleep:tilt-7175871cc674cce5\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.343s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.241s\n\u001b[34m  │ \u001b[0mDone in: 1.585s \n\nTaking a break...\nTen seconds later\n',
    },
    {
      name: "hello-world",
      directoriesWatched: [],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T10:59:56.300083-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:56.010435-04:00",
          finishTime: "2019-04-22T10:59:56.300082-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mhello-world\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 0.289s\n\u001b[34m  │ \u001b[0mDone in: 0.289s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9999/"],
      k8sResourceInfo: {
        podName: "hello-world-9f4c9b98b-cvxqn",
        podCreationTime: "2019-04-22T10:59:56-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: false,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mhello-world\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 0.289s\n\u001b[34m  │ \u001b[0mDone in: 0.289s \n\n",
    },
    {
      name: "tick",
      directoriesWatched: [],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T10:59:56.48933-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:56.300168-04:00",
          finishTime: "2019-04-22T10:59:56.489329-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mtick\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 0.188s\n\u001b[34m  │ \u001b[0mDone in: 0.188s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: null,
      k8sResourceInfo: {
        podName: "",
        podCreationTime: "0001-01-01T00:00:00Z",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "",
        podRestarts: 0,
        podLog: "",
      },
      runtimeStatus: "pending",
      isTiltfile: false,
      showBuildStatus: false,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mtick\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 0.188s\n\u001b[34m  │ \u001b[0mDone in: 0.188s \n\n",
    },
    {
      name: "k8s_yaml",
      directoriesWatched: [
        "Tiltfile",
        ".tiltignore",
        "tilt_option.json",
        "deploy/fe.yaml",
        "deploy/vigoda.yaml",
        "deploy/snack.yaml",
        "deploy/doggos.yaml",
        "deploy/fortune.yaml",
        "deploy/hypothesizer.yaml",
        "deploy/spoonerisms.yaml",
        "deploy/emoji.yaml",
        "deploy/words.yaml",
        "deploy/secrets.yaml",
        "deploy/job.yaml",
        "deploy/sleeper.yaml",
        "deploy/hello_world.yaml",
        "deploy/tick.yaml",
        "vigoda/Dockerfile",
        "snack/Dockerfile",
        "doggos/Dockerfile",
        "emoji/Dockerfile",
        "words/Dockerfile",
        "secrets/Dockerfile",
        "sleeper/Dockerfile",
        "sidecar/Dockerfile",
        "fe/Dockerfile",
        "hypothesizer/Dockerfile",
        "fortune/Dockerfile",
        "spoonerisms/Dockerfile",
      ],
      pathsWatched: null,
      lastDeployTime: "2019-04-22T10:59:56.007895-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:53.913447-04:00",
          finishTime: "2019-04-22T10:59:56.007894-04:00",
          log: "",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: null,
      yamlResourceInfo: {
        k8sResources: [
          "k8sRole-pod-reader",
          "k8sRoleBinding-read-pods",
          "k8sSecret-dan-servantes-stuff",
        ],
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: false,
      combinedLog: "",
    },
  ]
}

function oneResourceFailedToBuild(): any {
  return [
    {
      name: "snack",
      directoriesWatched: ["snack"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:04.242586-04:00",
      buildHistory: [
        {
          edits: ["main.go"],
          error: {},
          warnings: null,
          startTime: "2019-04-22T11:05:07.250689-04:00",
          finishTime: "2019-04-22T11:05:17.689819-04:00",
          log:
            "\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n    ╎   → # github.com/windmilleng/servantes/snack\nsrc/github.com/windmilleng/servantes/snack/main.go:21:17: syntax error: unexpected newline, expecting comma or }\n\n    ╎ ERROR IN: [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[31mERROR:\u001b[0m ImageBuild: executor failed running [/bin/sh -c go install github.com/windmilleng/servantes/snack]: exit code: 2\n",
        },
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:02.810268-04:00",
          finishTime: "2019-04-22T11:00:04.242583-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-13631d4ed09f1a05\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.241s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.190s\n\u001b[34m  │ \u001b[0mDone in: 1.431s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 1,
      pendingBuildEdits: ["main.go"],
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9002/"],
      k8sResourceInfo: {
        podName: "dan-snack-f885fb46f-d5z2t",
        podCreationTime: "2019-04-22T11:00:04-04:00",
        podUpdateStartTime: "2019-04-22T11:05:07.250689-04:00",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-13631d4ed09f1a05\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.241s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.190s\n\u001b[34m  │ \u001b[0mDone in: 1.431s \n\n2019/04/22 15:00:06 Starting Snack Service on :8083\n\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n    ╎   → # github.com/windmilleng/servantes/snack\nsrc/github.com/windmilleng/servantes/snack/main.go:21:17: syntax error: unexpected newline, expecting comma or }\n\n    ╎ ERROR IN: [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[31mERROR:\u001b[0m ImageBuild: executor failed running [/bin/sh -c go install github.com/windmilleng/servantes/snack]: exit code: 2\n",
    },
  ]
}

function oneResourceBuilding() {
  return [
    {
      name: "(Tiltfile)",
      directoriesWatched: null,
      pathsWatched: null,
      lastDeployTime: "2019-04-22T10:59:53.903047-04:00",
      buildHistory: [
        {
          edits: [
            "/Users/dan/go/src/github.com/windmilleng/servantes/Tiltfile",
          ],
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:53.574652-04:00",
          finishTime: "2019-04-22T10:59:53.903047-04:00",
          log:
            'Beginning Tiltfile execution\nRunning `"whoami"`\nRunning `"m4 -Dvarowner=dan \\"deploy/fe.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/vigoda.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/snack.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/doggos.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/fortune.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hypothesizer.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/spoonerisms.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/emoji.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/words.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/secrets.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/job.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/sleeper.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hello_world.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/tick.yaml\\""`\nSuccessfully loaded Tiltfile\n',
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: null,
      runtimeStatus: "ok",
      isTiltfile: true,
      showBuildStatus: false,
      combinedLog:
        'Beginning Tiltfile execution\nRunning `"whoami"`\nRunning `"m4 -Dvarowner=dan \\"deploy/fe.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/vigoda.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/snack.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/doggos.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/fortune.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hypothesizer.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/spoonerisms.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/emoji.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/words.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/secrets.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/job.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/sleeper.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hello_world.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/tick.yaml\\""`\nSuccessfully loaded Tiltfile\n',
    },
    {
      name: "fe",
      directoriesWatched: ["fe"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:01.337285-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T10:59:56.489417-04:00",
          finishTime: "2019-04-22T11:00:01.337284-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mfe\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/fe]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  RUN apt update && apt install -y unzip time make\n  \n  ENV PROTOC_VERSION 3.5.1\n  \n  RUN wget https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \\\n    unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \\\n    mv protoc/bin/protoc /usr/bin/protoc\n  \n  RUN go get github.com/golang/protobuf/protoc-gen-go\n  \n  ADD . /go/src/github.com/windmilleng/servantes/fe\n  RUN go install github.com/windmilleng/servantes/fe\n  ENTRYPOINT /go/bin/fe\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 24 MB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/6] FROM docker.io/library/golang:1.10\n    ╎ [2/6] RUN apt update && apt install -y unzip time make\n    ╎ [3/6] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc\n    ╎ [4/6] RUN go get github.com/golang/protobuf/protoc-gen-go\n    ╎ [5/6] ADD . /go/src/github.com/windmilleng/servantes/fe\n    ╎ [6/6] RUN go install github.com/windmilleng/servantes/fe\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/fe:tilt-2540b7769f4b0e45\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 4.628s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.218s\n\u001b[34m  │ \u001b[0mDone in: 4.847s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9000/"],
      k8sResourceInfo: {
        podName: "dan-fe-7cdc8f978f-vp94d",
        podCreationTime: "2019-04-22T11:00:01-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "2019/04/22 15:00:03 Starting Servantes FE on :8080\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mfe\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/fe]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  RUN apt update && apt install -y unzip time make\n  \n  ENV PROTOC_VERSION 3.5.1\n  \n  RUN wget https://github.com/google/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && \\\n    unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d protoc && \\\n    mv protoc/bin/protoc /usr/bin/protoc\n  \n  RUN go get github.com/golang/protobuf/protoc-gen-go\n  \n  ADD . /go/src/github.com/windmilleng/servantes/fe\n  RUN go install github.com/windmilleng/servantes/fe\n  ENTRYPOINT /go/bin/fe\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 24 MB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/6] FROM docker.io/library/golang:1.10\n    ╎ [2/6] RUN apt update && apt install -y unzip time make\n    ╎ [3/6] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &&   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &&   mv protoc/bin/protoc /usr/bin/protoc\n    ╎ [4/6] RUN go get github.com/golang/protobuf/protoc-gen-go\n    ╎ [5/6] ADD . /go/src/github.com/windmilleng/servantes/fe\n    ╎ [6/6] RUN go install github.com/windmilleng/servantes/fe\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/fe:tilt-2540b7769f4b0e45\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 4.628s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.218s\n\u001b[34m  │ \u001b[0mDone in: 4.847s \n\n2019/04/22 15:00:03 Starting Servantes FE on :8080\n",
    },
    {
      name: "vigoda",
      directoriesWatched: ["vigoda"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:00:02.810113-04:00",
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:00:01.337359-04:00",
          finishTime: "2019-04-22T11:00:02.810112-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0mvigoda\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/vigoda]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/vigoda\n  RUN go install github.com/windmilleng/servantes/vigoda\n  \n  ENTRYPOINT /go/bin/vigoda\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 8.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/vigoda\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/vigoda:tilt-2d369271c8091f68\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.283s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.189s\n\u001b[34m  │ \u001b[0mDone in: 1.472s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9001/"],
      k8sResourceInfo: {
        podName: "dan-vigoda-67d79bd8d5-w77q4",
        podCreationTime: "2019-04-22T11:00:02-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog:
          "2019/04/22 15:00:04 Starting Vigoda Health Check Service on :8081\n2019/04/22 15:00:06 Server status: All good\n2019/04/22 15:00:08 Server status: All good\n2019/04/22 15:00:10 Server status: All good\n2019/04/22 15:00:12 Server status: All good\n2019/04/22 15:00:14 Server status: All good\n2019/04/22 15:00:16 Server status: All good\n2019/04/22 15:00:18 Server status: All good\n2019/04/22 15:00:20 Server status: All good\n2019/04/22 15:00:22 Server status: All good\n2019/04/22 15:00:24 Server status: All good\n2019/04/22 15:00:26 Server status: All good\n2019/04/22 15:00:28 Server status: All good\n2019/04/22 15:00:30 Server status: All good\n2019/04/22 15:00:32 Server status: All good\n2019/04/22 15:00:34 Server status: All good\n2019/04/22 15:00:36 Server status: All good\n2019/04/22 15:00:38 Server status: All good\n2019/04/22 15:00:40 Server status: All good\n2019/04/22 15:00:42 Server status: All good\n2019/04/22 15:00:44 Server status: All good\n2019/04/22 15:00:46 Server status: All good\n2019/04/22 15:00:48 Server status: All good\n2019/04/22 15:00:50 Server status: All good\n2019/04/22 15:00:52 Server status: All good\n2019/04/22 15:00:54 Server status: All good\n2019/04/22 15:00:56 Server status: All good\n2019/04/22 15:00:58 Server status: All good\n2019/04/22 15:01:00 Server status: All good\n2019/04/22 15:01:02 Server status: All good\n2019/04/22 15:01:04 Server status: All good\n2019/04/22 15:01:06 Server status: All good\n2019/04/22 15:01:08 Server status: All good\n2019/04/22 15:01:10 Server status: All good\n2019/04/22 15:01:12 Server status: All good\n2019/04/22 15:01:14 Server status: All good\n2019/04/22 15:01:16 Server status: All good\n2019/04/22 15:01:18 Server status: All good\n2019/04/22 15:01:20 Server status: All good\n2019/04/22 15:01:22 Server status: All good\n2019/04/22 15:01:24 Server status: All good\n2019/04/22 15:01:26 Server status: All good\n2019/04/22 15:01:28 Server status: All good\n2019/04/22 15:01:30 Server status: All good\n2019/04/22 15:01:32 Server status: All good\n2019/04/22 15:01:34 Server status: All good\n2019/04/22 15:01:36 Server status: All good\n2019/04/22 15:01:38 Server status: All good\n2019/04/22 15:01:40 Server status: All good\n2019/04/22 15:01:42 Server status: All good\n2019/04/22 15:01:44 Server status: All good\n2019/04/22 15:01:46 Server status: All good\n2019/04/22 15:01:48 Server status: All good\n2019/04/22 15:01:50 Server status: All good\n2019/04/22 15:01:52 Server status: All good\n2019/04/22 15:01:54 Server status: All good\n2019/04/22 15:01:56 Server status: All good\n2019/04/22 15:01:58 Server status: All good\n2019/04/22 15:02:00 Server status: All good\n2019/04/22 15:02:02 Server status: All good\n2019/04/22 15:02:04 Server status: All good\n2019/04/22 15:02:06 Server status: All good\n2019/04/22 15:02:08 Server status: All good\n2019/04/22 15:02:10 Server status: All good\n2019/04/22 15:02:12 Server status: All good\n2019/04/22 15:02:14 Server status: All good\n2019/04/22 15:02:16 Server status: All good\n2019/04/22 15:02:18 Server status: All good\n2019/04/22 15:02:20 Server status: All good\n2019/04/22 15:02:22 Server status: All good\n2019/04/22 15:02:24 Server status: All good\n2019/04/22 15:02:26 Server status: All good\n2019/04/22 15:02:28 Server status: All good\n2019/04/22 15:02:30 Server status: All good\n2019/04/22 15:02:32 Server status: All good\n2019/04/22 15:02:34 Server status: All good\n2019/04/22 15:02:36 Server status: All good\n2019/04/22 15:02:38 Server status: All good\n2019/04/22 15:02:40 Server status: All good\n2019/04/22 15:02:42 Server status: All good\n2019/04/22 15:02:44 Server status: All good\n2019/04/22 15:02:46 Server status: All good\n2019/04/22 15:02:48 Server status: All good\n2019/04/22 15:02:50 Server status: All good\n2019/04/22 15:02:52 Server status: All good\n2019/04/22 15:02:54 Server status: All good\n2019/04/22 15:02:56 Server status: All good\n2019/04/22 15:02:58 Server status: All good\n2019/04/22 15:03:00 Server status: All good\n2019/04/22 15:03:02 Server status: All good\n2019/04/22 15:03:04 Server status: All good\n2019/04/22 15:03:06 Server status: All good\n2019/04/22 15:03:08 Server status: All good\n2019/04/22 15:03:10 Server status: All good\n2019/04/22 15:03:12 Server status: All good\n2019/04/22 15:03:14 Server status: All good\n2019/04/22 15:03:16 Server status: All good\n2019/04/22 15:03:18 Server status: All good\n2019/04/22 15:03:20 Server status: All good\n2019/04/22 15:03:22 Server status: All good\n2019/04/22 15:03:24 Server status: All good\n2019/04/22 15:03:26 Server status: All good\n2019/04/22 15:03:28 Server status: All good\n2019/04/22 15:03:30 Server status: All good\n2019/04/22 15:03:32 Server status: All good\n2019/04/22 15:03:34 Server status: All good\n2019/04/22 15:03:36 Server status: All good\n2019/04/22 15:03:38 Server status: All good\n2019/04/22 15:03:40 Server status: All good\n2019/04/22 15:03:42 Server status: All good\n2019/04/22 15:03:44 Server status: All good\n2019/04/22 15:03:46 Server status: All good\n2019/04/22 15:03:48 Server status: All good\n2019/04/22 15:03:50 Server status: All good\n2019/04/22 15:03:52 Server status: All good\n2019/04/22 15:03:54 Server status: All good\n2019/04/22 15:03:56 Server status: All good\n2019/04/22 15:03:58 Server status: All good\n2019/04/22 15:04:00 Server status: All good\n2019/04/22 15:04:02 Server status: All good\n2019/04/22 15:04:04 Server status: All good\n2019/04/22 15:04:06 Server status: All good\n2019/04/22 15:04:08 Server status: All good\n2019/04/22 15:04:10 Server status: All good\n2019/04/22 15:04:12 Server status: All good\n2019/04/22 15:04:14 Server status: All good\n2019/04/22 15:04:16 Server status: All good\n2019/04/22 15:04:18 Server status: All good\n2019/04/22 15:04:20 Server status: All good\n2019/04/22 15:04:22 Server status: All good\n2019/04/22 15:04:24 Server status: All good\n2019/04/22 15:04:26 Server status: All good\n2019/04/22 15:04:28 Server status: All good\n2019/04/22 15:04:30 Server status: All good\n2019/04/22 15:04:32 Server status: All good\n2019/04/22 15:04:34 Server status: All good\n2019/04/22 15:04:36 Server status: All good\n2019/04/22 15:04:38 Server status: All good\n2019/04/22 15:04:40 Server status: All good\n2019/04/22 15:04:42 Server status: All good\n2019/04/22 15:04:44 Server status: All good\n2019/04/22 15:04:46 Server status: All good\n2019/04/22 15:04:48 Server status: All good\n2019/04/22 15:04:50 Server status: All good\n2019/04/22 15:04:52 Server status: All good\n2019/04/22 15:04:54 Server status: All good\n2019/04/22 15:04:56 Server status: All good\n2019/04/22 15:04:58 Server status: All good\n2019/04/22 15:05:00 Server status: All good\n2019/04/22 15:05:02 Server status: All good\n2019/04/22 15:05:04 Server status: All good\n2019/04/22 15:05:06 Server status: All good\n2019/04/22 15:05:08 Server status: All good\n2019/04/22 15:05:10 Server status: All good\n2019/04/22 15:05:12 Server status: All good\n2019/04/22 15:05:14 Server status: All good\n2019/04/22 15:05:16 Server status: All good\n2019/04/22 15:05:18 Server status: All good\n2019/04/22 15:05:20 Server status: All good\n2019/04/22 15:05:22 Server status: All good\n2019/04/22 15:05:24 Server status: All good\n2019/04/22 15:05:26 Server status: All good\n2019/04/22 15:05:28 Server status: All good\n2019/04/22 15:05:30 Server status: All good\n2019/04/22 15:05:32 Server status: All good\n2019/04/22 15:05:34 Server status: All good\n2019/04/22 15:05:36 Server status: All good\n2019/04/22 15:05:38 Server status: All good\n2019/04/22 15:05:40 Server status: All good\n2019/04/22 15:05:42 Server status: All good\n2019/04/22 15:05:44 Server status: All good\n2019/04/22 15:05:46 Server status: All good\n2019/04/22 15:05:48 Server status: All good\n2019/04/22 15:05:50 Server status: All good\n2019/04/22 15:05:52 Server status: All good\n2019/04/22 15:05:54 Server status: All good\n2019/04/22 15:05:56 Server status: All good\n2019/04/22 15:05:58 Server status: All good\n2019/04/22 15:06:00 Server status: All good\n2019/04/22 15:06:02 Server status: All good\n2019/04/22 15:06:04 Server status: All good\n2019/04/22 15:06:06 Server status: All good\n2019/04/22 15:06:08 Server status: All good\n2019/04/22 15:06:10 Server status: All good\n2019/04/22 15:06:12 Server status: All good\n2019/04/22 15:06:14 Server status: All good\n2019/04/22 15:06:16 Server status: All good\n2019/04/22 15:06:18 Server status: All good\n2019/04/22 15:06:20 Server status: All good\n2019/04/22 15:06:22 Server status: All good\n2019/04/22 15:06:24 Server status: All good\n2019/04/22 15:06:26 Server status: All good\n2019/04/22 15:06:28 Server status: All good\n2019/04/22 15:06:30 Server status: All good\n2019/04/22 15:06:32 Server status: All good\n2019/04/22 15:06:34 Server status: All good\n2019/04/22 15:06:36 Server status: All good\n2019/04/22 15:06:38 Server status: All good\n2019/04/22 15:06:40 Server status: All good\n2019/04/22 15:06:42 Server status: All good\n2019/04/22 15:06:45 Server status: All good\n2019/04/22 15:06:47 Server status: All good\n2019/04/22 15:06:49 Server status: All good\n2019/04/22 15:06:51 Server status: All good\n2019/04/22 15:06:53 Server status: All good\n2019/04/22 15:06:55 Server status: All good\n2019/04/22 15:06:57 Server status: All good\n2019/04/22 15:06:59 Server status: All good\n2019/04/22 15:07:01 Server status: All good\n2019/04/22 15:07:03 Server status: All good\n2019/04/22 15:07:05 Server status: All good\n2019/04/22 15:07:07 Server status: All good\n2019/04/22 15:07:09 Server status: All good\n2019/04/22 15:07:11 Server status: All good\n2019/04/22 15:07:13 Server status: All good\n2019/04/22 15:07:15 Server status: All good\n2019/04/22 15:07:17 Server status: All good\n2019/04/22 15:07:19 Server status: All good\n2019/04/22 15:07:21 Server status: All good\n2019/04/22 15:07:23 Server status: All good\n2019/04/22 15:07:25 Server status: All good\n2019/04/22 15:07:27 Server status: All good\n2019/04/22 15:07:29 Server status: All good\n2019/04/22 15:07:31 Server status: All good\n2019/04/22 15:07:33 Server status: All good\n2019/04/22 15:07:35 Server status: All good\n2019/04/22 15:07:37 Server status: All good\n2019/04/22 15:07:39 Server status: All good\n2019/04/22 15:07:41 Server status: All good\n2019/04/22 15:07:43 Server status: All good\n2019/04/22 15:07:45 Server status: All good\n2019/04/22 15:07:47 Server status: All good\n2019/04/22 15:07:49 Server status: All good\n2019/04/22 15:07:51 Server status: All good\n2019/04/22 15:07:53 Server status: All good\n2019/04/22 15:07:55 Server status: All good\n2019/04/22 15:07:57 Server status: All good\n2019/04/22 15:07:59 Server status: All good\n2019/04/22 15:08:01 Server status: All good\n2019/04/22 15:08:03 Server status: All good\n2019/04/22 15:08:05 Server status: All good\n2019/04/22 15:08:07 Server status: All good\n2019/04/22 15:08:09 Server status: All good\n2019/04/22 15:08:11 Server status: All good\n2019/04/22 15:08:13 Server status: All good\n2019/04/22 15:08:15 Server status: All good\n2019/04/22 15:08:17 Server status: All good\n2019/04/22 15:08:19 Server status: All good\n2019/04/22 15:08:21 Server status: All good\n2019/04/22 15:08:23 Server status: All good\n2019/04/22 15:08:25 Server status: All good\n2019/04/22 15:08:27 Server status: All good\n2019/04/22 15:08:29 Server status: All good\n2019/04/22 15:08:31 Server status: All good\n2019/04/22 15:08:33 Server status: All good\n2019/04/22 15:08:35 Server status: All good\n2019/04/22 15:08:37 Server status: All good\n2019/04/22 15:08:39 Server status: All good\n2019/04/22 15:08:41 Server status: All good\n2019/04/22 15:08:43 Server status: All good\n2019/04/22 15:08:45 Server status: All good\n2019/04/22 15:08:47 Server status: All good\n2019/04/22 15:08:49 Server status: All good\n2019/04/22 15:08:51 Server status: All good\n2019/04/22 15:08:53 Server status: All good\n2019/04/22 15:08:55 Server status: All good\n2019/04/22 15:08:57 Server status: All good\n2019/04/22 15:08:59 Server status: All good\n2019/04/22 15:09:01 Server status: All good\n2019/04/22 15:09:03 Server status: All good\n2019/04/22 15:09:05 Server status: All good\n2019/04/22 15:09:07 Server status: All good\n2019/04/22 15:09:09 Server status: All good\n2019/04/22 15:09:11 Server status: All good\n2019/04/22 15:09:13 Server status: All good\n2019/04/22 15:09:15 Server status: All good\n2019/04/22 15:09:17 Server status: All good\n2019/04/22 15:09:19 Server status: All good\n2019/04/22 15:09:21 Server status: All good\n2019/04/22 15:09:23 Server status: All good\n2019/04/22 15:09:25 Server status: All good\n2019/04/22 15:09:27 Server status: All good\n2019/04/22 15:09:29 Server status: All good\n2019/04/22 15:09:31 Server status: All good\n2019/04/22 15:09:33 Server status: All good\n2019/04/22 15:09:35 Server status: All good\n2019/04/22 15:09:37 Server status: All good\n2019/04/22 15:09:39 Server status: All good\n2019/04/22 15:09:41 Server status: All good\n2019/04/22 15:09:43 Server status: All good\n2019/04/22 15:09:45 Server status: All good\n2019/04/22 15:09:47 Server status: All good\n2019/04/22 15:09:49 Server status: All good\n2019/04/22 15:09:51 Server status: All good\n2019/04/22 15:09:53 Server status: All good\n2019/04/22 15:09:55 Server status: All good\n2019/04/22 15:09:57 Server status: All good\n2019/04/22 15:09:59 Server status: All good\n2019/04/22 15:10:01 Server status: All good\n2019/04/22 15:10:03 Server status: All good\n2019/04/22 15:10:05 Server status: All good\n2019/04/22 15:10:07 Server status: All good\n2019/04/22 15:10:09 Server status: All good\n2019/04/22 15:10:11 Server status: All good\n2019/04/22 15:10:13 Server status: All good\n2019/04/22 15:10:15 Server status: All good\n2019/04/22 15:10:17 Server status: All good\n2019/04/22 15:10:19 Server status: All good\n2019/04/22 15:10:20 Server status: All good\n2019/04/22 15:10:22 Server status: All good\n2019/04/22 15:10:24 Server status: All good\n2019/04/22 15:10:26 Server status: All good\n2019/04/22 15:10:29 Server status: All good\n2019/04/22 15:10:31 Server status: All good\n2019/04/22 15:10:33 Server status: All good\n2019/04/22 15:10:35 Server status: All good\n2019/04/22 15:10:37 Server status: All good\n2019/04/22 15:10:39 Server status: All good\n2019/04/22 15:10:41 Server status: All good\n2019/04/22 15:10:43 Server status: All good\n2019/04/22 15:10:45 Server status: All good\n2019/04/22 15:10:47 Server status: All good\n2019/04/22 15:10:49 Server status: All good\n2019/04/22 15:10:50 Server status: All good\n2019/04/22 15:10:52 Server status: All good\n2019/04/22 15:10:54 Server status: All good\n2019/04/22 15:10:56 Server status: All good\n2019/04/22 15:10:58 Server status: All good\n2019/04/22 15:11:00 Server status: All good\n2019/04/22 15:11:02 Server status: All good\n2019/04/22 15:11:04 Server status: All good\n2019/04/22 15:11:06 Server status: All good\n2019/04/22 15:11:08 Server status: All good\n2019/04/22 15:11:10 Server status: All good\n2019/04/22 15:11:12 Server status: All good\n2019/04/22 15:11:14 Server status: All good\n2019/04/22 15:11:17 Server status: All good\n2019/04/22 15:11:19 Server status: All good\n2019/04/22 15:11:20 Server status: All good\n2019/04/22 15:11:22 Server status: All good\n2019/04/22 15:11:24 Server status: All good\n2019/04/22 15:11:26 Server status: All good\n2019/04/22 15:11:28 Server status: All good\n2019/04/22 15:11:30 Server status: All good\n2019/04/22 15:11:32 Server status: All good\n2019/04/22 15:11:34 Server status: All good\n2019/04/22 15:11:36 Server status: All good\n2019/04/22 15:11:38 Server status: All good\n2019/04/22 15:11:40 Server status: All good\n2019/04/22 15:11:42 Server status: All good\n2019/04/22 15:11:44 Server status: All good\n2019/04/22 15:11:46 Server status: All good\n2019/04/22 15:11:48 Server status: All good\n2019/04/22 15:11:50 Server status: All good\n2019/04/22 15:11:52 Server status: All good\n2019/04/22 15:11:54 Server status: All good\n2019/04/22 15:11:56 Server status: All good\n2019/04/22 15:11:58 Server status: All good\n2019/04/22 15:12:00 Server status: All good\n2019/04/22 15:12:02 Server status: All good\n2019/04/22 15:12:04 Server status: All good\n2019/04/22 15:12:06 Server status: All good\n2019/04/22 15:12:08 Server status: All good\n2019/04/22 15:12:10 Server status: All good\n2019/04/22 15:12:12 Server status: All good\n2019/04/22 15:12:14 Server status: All good\n2019/04/22 15:12:16 Server status: All good\n2019/04/22 15:12:18 Server status: All good\n2019/04/22 15:12:20 Server status: All good\n2019/04/22 15:12:22 Server status: All good\n2019/04/22 15:12:24 Server status: All good\n2019/04/22 15:12:26 Server status: All good\n2019/04/22 15:12:28 Server status: All good\n2019/04/22 15:12:30 Server status: All good\n2019/04/22 15:12:32 Server status: All good\n2019/04/22 15:12:34 Server status: All good\n2019/04/22 15:12:36 Server status: All good\n2019/04/22 15:12:38 Server status: All good\n2019/04/22 15:12:40 Server status: All good\n2019/04/22 15:12:42 Server status: All good\n2019/04/22 15:12:44 Server status: All good\n2019/04/22 15:12:46 Server status: All good\n2019/04/22 15:12:48 Server status: All good\n2019/04/22 15:12:50 Server status: All good\n2019/04/22 15:12:52 Server status: All good\n2019/04/22 15:12:54 Server status: All good\n2019/04/22 15:12:56 Server status: All good\n2019/04/22 15:12:58 Server status: All good\n2019/04/22 15:13:00 Server status: All good\n2019/04/22 15:13:02 Server status: All good\n2019/04/22 15:13:04 Server status: All good\n2019/04/22 15:13:06 Server status: All good\n2019/04/22 15:13:08 Server status: All good\n2019/04/22 15:13:10 Server status: All good\n2019/04/22 15:13:12 Server status: All good\n2019/04/22 15:13:14 Server status: All good\n2019/04/22 15:13:16 Server status: All good\n2019/04/22 15:13:18 Server status: All good\n2019/04/22 15:13:20 Server status: All good\n2019/04/22 15:13:22 Server status: All good\n2019/04/22 15:13:24 Server status: All good\n2019/04/22 15:13:26 Server status: All good\n2019/04/22 15:13:28 Server status: All good\n2019/04/22 15:13:30 Server status: All good\n2019/04/22 15:13:32 Server status: All good\n2019/04/22 15:13:34 Server status: All good\n2019/04/22 15:13:36 Server status: All good\n2019/04/22 15:13:38 Server status: All good\n2019/04/22 15:13:40 Server status: All good\n2019/04/22 15:13:42 Server status: All good\n2019/04/22 15:13:44 Server status: All good\n2019/04/22 15:13:46 Server status: All good\n2019/04/22 15:13:48 Server status: All good\n2019/04/22 15:13:50 Server status: All good\n2019/04/22 15:13:52 Server status: All good\n2019/04/22 15:13:54 Server status: All good\n2019/04/22 15:13:56 Server status: All good\n2019/04/22 15:13:58 Server status: All good\n2019/04/22 15:14:00 Server status: All good\n2019/04/22 15:14:02 Server status: All good\n2019/04/22 15:14:04 Server status: All good\n2019/04/22 15:14:06 Server status: All good\n2019/04/22 15:14:08 Server status: All good\n2019/04/22 15:14:10 Server status: All good\n2019/04/22 15:14:12 Server status: All good\n2019/04/22 15:14:14 Server status: All good\n2019/04/22 15:14:16 Server status: All good\n2019/04/22 15:14:18 Server status: All good\n2019/04/22 15:14:20 Server status: All good\n2019/04/22 15:14:22 Server status: All good\n2019/04/22 15:14:24 Server status: All good\n2019/04/22 15:14:26 Server status: All good\n2019/04/22 15:14:28 Server status: All good\n2019/04/22 15:14:30 Server status: All good\n2019/04/22 15:14:32 Server status: All good\n2019/04/22 15:14:34 Server status: All good\n2019/04/22 15:14:36 Server status: All good\n2019/04/22 15:14:38 Server status: All good\n2019/04/22 15:14:40 Server status: All good\n2019/04/22 15:14:42 Server status: All good\n2019/04/22 15:14:44 Server status: All good\n2019/04/22 15:14:46 Server status: All good\n2019/04/22 15:14:48 Server status: All good\n2019/04/22 15:14:50 Server status: All good\n2019/04/22 15:14:52 Server status: All good\n2019/04/22 15:14:54 Server status: All good\n2019/04/22 15:14:56 Server status: All good\n2019/04/22 15:14:58 Server status: All good\n2019/04/22 15:15:00 Server status: All good\n2019/04/22 15:15:02 Server status: All good\n2019/04/22 15:15:04 Server status: All good\n2019/04/22 15:15:06 Server status: All good\n2019/04/22 15:15:08 Server status: All good\n2019/04/22 15:15:10 Server status: All good\n2019/04/22 15:15:12 Server status: All good\n2019/04/22 15:15:14 Server status: All good\n2019/04/22 15:15:16 Server status: All good\n2019/04/22 15:15:18 Server status: All good\n2019/04/22 15:15:20 Server status: All good\n2019/04/22 15:15:22 Server status: All good\n2019/04/22 15:15:24 Server status: All good\n2019/04/22 15:15:26 Server status: All good\n2019/04/22 15:15:28 Server status: All good\n2019/04/22 15:15:30 Server status: All good\n2019/04/22 15:15:32 Server status: All good\n2019/04/22 15:15:34 Server status: All good\n2019/04/22 15:15:36 Server status: All good\n2019/04/22 15:15:38 Server status: All good\n2019/04/22 15:15:40 Server status: All good\n2019/04/22 15:15:42 Server status: All good\n2019/04/22 15:15:44 Server status: All good\n2019/04/22 15:15:46 Server status: All good\n2019/04/22 15:15:48 Server status: All good\n2019/04/22 15:15:50 Server status: All good\n2019/04/22 15:15:52 Server status: All good\n2019/04/22 15:15:54 Server status: All good\n2019/04/22 15:15:56 Server status: All good\n2019/04/22 15:15:58 Server status: All good\n2019/04/22 15:16:00 Server status: All good\n2019/04/22 15:16:02 Server status: All good\n2019/04/22 15:16:04 Server status: All good\n2019/04/22 15:16:06 Server status: All good\n2019/04/22 15:16:08 Server status: All good\n2019/04/22 15:16:10 Server status: All good\n2019/04/22 15:16:12 Server status: All good\n2019/04/22 15:16:14 Server status: All good\n2019/04/22 15:16:16 Server status: All good\n2019/04/22 15:16:18 Server status: All good\n2019/04/22 15:16:20 Server status: All good\n2019/04/22 15:16:22 Server status: All good\n2019/04/22 15:16:24 Server status: All good\n2019/04/22 15:16:26 Server status: All good\n2019/04/22 15:16:28 Server status: All good\n2019/04/22 15:16:30 Server status: All good\n2019/04/22 15:16:32 Server status: All good\n2019/04/22 15:16:34 Server status: All good\n2019/04/22 15:16:36 Server status: All good\n2019/04/22 15:16:38 Server status: All good\n2019/04/22 15:16:40 Server status: All good\n2019/04/22 15:16:42 Server status: All good\n2019/04/22 15:16:44 Server status: All good\n2019/04/22 15:16:46 Server status: All good\n2019/04/22 15:16:48 Server status: All good\n2019/04/22 15:16:50 Server status: All good\n2019/04/22 15:16:52 Server status: All good\n2019/04/22 15:16:54 Server status: All good\n2019/04/22 15:16:56 Server status: All good\n2019/04/22 15:16:58 Server status: All good\n2019/04/22 15:17:00 Server status: All good\n2019/04/22 15:17:02 Server status: All good\n2019/04/22 15:17:04 Server status: All good\n2019/04/22 15:17:06 Server status: All good\n2019/04/22 15:17:08 Server status: All good\n2019/04/22 15:17:10 Server status: All good\n2019/04/22 15:17:12 Server status: All good\n2019/04/22 15:17:14 Server status: All good\n2019/04/22 15:17:16 Server status: All good\n2019/04/22 15:17:18 Server status: All good\n2019/04/22 15:17:20 Server status: All good\n2019/04/22 15:17:22 Server status: All good\n2019/04/22 15:17:24 Server status: All good\n2019/04/22 15:17:26 Server status: All good\n2019/04/22 15:17:28 Server status: All good\n2019/04/22 15:17:30 Server status: All good\n2019/04/22 15:17:32 Server status: All good\n2019/04/22 15:17:34 Server status: All good\n2019/04/22 15:17:36 Server status: All good\n2019/04/22 15:17:38 Server status: All good\n2019/04/22 15:17:40 Server status: All good\n2019/04/22 15:17:42 Server status: All good\n2019/04/22 15:17:44 Server status: All good\n2019/04/22 15:17:46 Server status: All good\n2019/04/22 15:17:48 Server status: All good\n2019/04/22 15:17:50 Server status: All good\n2019/04/22 15:17:52 Server status: All good\n2019/04/22 15:17:54 Server status: All good\n2019/04/22 15:17:56 Server status: All good\n2019/04/22 15:17:58 Server status: All good\n2019/04/22 15:18:00 Server status: All good\n2019/04/22 15:18:02 Server status: All good\n2019/04/22 15:18:04 Server status: All good\n2019/04/22 15:18:06 Server status: All good\n2019/04/22 15:18:08 Server status: All good\n2019/04/22 15:18:10 Server status: All good\n2019/04/22 15:18:12 Server status: All good\n2019/04/22 15:18:14 Server status: All good\n2019/04/22 15:18:16 Server status: All good\n2019/04/22 15:18:18 Server status: All good\n2019/04/22 15:18:20 Server status: All good\n2019/04/22 15:18:22 Server status: All good\n2019/04/22 15:18:24 Server status: All good\n2019/04/22 15:18:26 Server status: All good\n2019/04/22 15:18:28 Server status: All good\n2019/04/22 15:18:30 Server status: All good\n2019/04/22 15:18:32 Server status: All good\n2019/04/22 15:18:34 Server status: All good\n2019/04/22 15:18:36 Server status: All good\n2019/04/22 15:18:38 Server status: All good\n2019/04/22 15:18:40 Server status: All good\n2019/04/22 15:18:42 Server status: All good\n2019/04/22 15:18:44 Server status: All good\n2019/04/22 15:18:46 Server status: All good\n2019/04/22 15:18:48 Server status: All good\n2019/04/22 15:18:50 Server status: All good\n2019/04/22 15:18:52 Server status: All good\n2019/04/22 15:18:54 Server status: All good\n2019/04/22 15:18:56 Server status: All good\n2019/04/22 15:18:58 Server status: All good\n2019/04/22 15:19:00 Server status: All good\n2019/04/22 15:19:02 Server status: All good\n2019/04/22 15:19:04 Server status: All good\n2019/04/22 15:19:06 Server status: All good\n2019/04/22 15:19:08 Server status: All good\n2019/04/22 15:19:10 Server status: All good\n2019/04/22 15:19:12 Server status: All good\n2019/04/22 15:19:14 Server status: All good\n2019/04/22 15:19:16 Server status: All good\n2019/04/22 15:19:18 Server status: All good\n2019/04/22 15:19:20 Server status: All good\n2019/04/22 15:19:22 Server status: All good\n2019/04/22 15:19:24 Server status: All good\n2019/04/22 15:19:26 Server status: All good\n2019/04/22 15:19:28 Server status: All good\n2019/04/22 15:19:30 Server status: All good\n2019/04/22 15:19:32 Server status: All good\n2019/04/22 15:19:34 Server status: All good\n2019/04/22 15:19:36 Server status: All good\n2019/04/22 15:19:38 Server status: All good\n2019/04/22 15:19:40 Server status: All good\n2019/04/22 15:19:42 Server status: All good\n2019/04/22 15:19:44 Server status: All good\n2019/04/22 15:19:46 Server status: All good\n2019/04/22 15:19:48 Server status: All good\n2019/04/22 15:19:50 Server status: All good\n2019/04/22 15:19:52 Server status: All good\n2019/04/22 15:19:54 Server status: All good\n2019/04/22 15:19:56 Server status: All good\n2019/04/22 15:19:58 Server status: All good\n2019/04/22 15:20:00 Server status: All good\n2019/04/22 15:20:02 Server status: All good\n2019/04/22 15:20:04 Server status: All good\n2019/04/22 15:20:06 Server status: All good\n2019/04/22 15:20:08 Server status: All good\n2019/04/22 15:20:10 Server status: All good\n2019/04/22 15:20:12 Server status: All good\n2019/04/22 15:20:14 Server status: All good\n2019/04/22 15:20:16 Server status: All good\n2019/04/22 15:20:18 Server status: All good\n2019/04/22 15:20:20 Server status: All good\n2019/04/22 15:20:22 Server status: All good\n2019/04/22 15:20:24 Server status: All good\n2019/04/22 15:20:26 Server status: All good\n2019/04/22 15:20:28 Server status: All good\n2019/04/22 15:20:30 Server status: All good\n2019/04/22 15:20:32 Server status: All good\n2019/04/22 15:20:34 Server status: All good\n2019/04/22 15:20:36 Server status: All good\n2019/04/22 15:20:38 Server status: All good\n2019/04/22 15:20:40 Server status: All good\n2019/04/22 15:20:42 Server status: All good\n2019/04/22 15:20:44 Server status: All good\n2019/04/22 15:20:46 Server status: All good\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0mvigoda\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/vigoda]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/vigoda\n  RUN go install github.com/windmilleng/servantes/vigoda\n  \n  ENTRYPOINT /go/bin/vigoda\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 8.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/vigoda\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/vigoda:tilt-2d369271c8091f68\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.283s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.189s\n\u001b[34m  │ \u001b[0mDone in: 1.472s \n\n2019/04/22 15:00:04 Starting Vigoda Health Check Service on :8081\n2019/04/22 15:00:06 Server status: All good\n2019/04/22 15:00:08 Server status: All good\n2019/04/22 15:00:10 Server status: All good\n2019/04/22 15:00:12 Server status: All good\n2019/04/22 15:00:14 Server status: All good\n2019/04/22 15:00:16 Server status: All good\n2019/04/22 15:00:18 Server status: All good\n2019/04/22 15:00:20 Server status: All good\n2019/04/22 15:00:22 Server status: All good\n2019/04/22 15:00:24 Server status: All good\n2019/04/22 15:00:26 Server status: All good\n2019/04/22 15:00:28 Server status: All good\n2019/04/22 15:00:30 Server status: All good\n2019/04/22 15:00:32 Server status: All good\n2019/04/22 15:00:34 Server status: All good\n2019/04/22 15:00:36 Server status: All good\n2019/04/22 15:00:38 Server status: All good\n2019/04/22 15:00:40 Server status: All good\n2019/04/22 15:00:42 Server status: All good\n2019/04/22 15:00:44 Server status: All good\n2019/04/22 15:00:46 Server status: All good\n2019/04/22 15:00:48 Server status: All good\n2019/04/22 15:00:50 Server status: All good\n2019/04/22 15:00:52 Server status: All good\n2019/04/22 15:00:54 Server status: All good\n2019/04/22 15:00:56 Server status: All good\n2019/04/22 15:00:58 Server status: All good\n2019/04/22 15:01:00 Server status: All good\n2019/04/22 15:01:02 Server status: All good\n2019/04/22 15:01:04 Server status: All good\n2019/04/22 15:01:06 Server status: All good\n2019/04/22 15:01:08 Server status: All good\n2019/04/22 15:01:10 Server status: All good\n2019/04/22 15:01:12 Server status: All good\n2019/04/22 15:01:14 Server status: All good\n2019/04/22 15:01:16 Server status: All good\n2019/04/22 15:01:18 Server status: All good\n2019/04/22 15:01:20 Server status: All good\n2019/04/22 15:01:22 Server status: All good\n2019/04/22 15:01:24 Server status: All good\n2019/04/22 15:01:26 Server status: All good\n2019/04/22 15:01:28 Server status: All good\n2019/04/22 15:01:30 Server status: All good\n2019/04/22 15:01:32 Server status: All good\n2019/04/22 15:01:34 Server status: All good\n2019/04/22 15:01:36 Server status: All good\n2019/04/22 15:01:38 Server status: All good\n2019/04/22 15:01:40 Server status: All good\n2019/04/22 15:01:42 Server status: All good\n2019/04/22 15:01:44 Server status: All good\n2019/04/22 15:01:46 Server status: All good\n2019/04/22 15:01:48 Server status: All good\n2019/04/22 15:01:50 Server status: All good\n2019/04/22 15:01:52 Server status: All good\n2019/04/22 15:01:54 Server status: All good\n2019/04/22 15:01:56 Server status: All good\n2019/04/22 15:01:58 Server status: All good\n2019/04/22 15:02:00 Server status: All good\n2019/04/22 15:02:02 Server status: All good\n2019/04/22 15:02:04 Server status: All good\n2019/04/22 15:02:06 Server status: All good\n2019/04/22 15:02:08 Server status: All good\n2019/04/22 15:02:10 Server status: All good\n2019/04/22 15:02:12 Server status: All good\n2019/04/22 15:02:14 Server status: All good\n2019/04/22 15:02:16 Server status: All good\n2019/04/22 15:02:18 Server status: All good\n2019/04/22 15:02:20 Server status: All good\n2019/04/22 15:02:22 Server status: All good\n2019/04/22 15:02:24 Server status: All good\n2019/04/22 15:02:26 Server status: All good\n2019/04/22 15:02:28 Server status: All good\n2019/04/22 15:02:30 Server status: All good\n2019/04/22 15:02:32 Server status: All good\n2019/04/22 15:02:34 Server status: All good\n2019/04/22 15:02:36 Server status: All good\n2019/04/22 15:02:38 Server status: All good\n2019/04/22 15:02:40 Server status: All good\n2019/04/22 15:02:42 Server status: All good\n2019/04/22 15:02:44 Server status: All good\n2019/04/22 15:02:46 Server status: All good\n2019/04/22 15:02:48 Server status: All good\n2019/04/22 15:02:50 Server status: All good\n2019/04/22 15:02:52 Server status: All good\n2019/04/22 15:02:54 Server status: All good\n2019/04/22 15:02:56 Server status: All good\n2019/04/22 15:02:58 Server status: All good\n2019/04/22 15:03:00 Server status: All good\n2019/04/22 15:03:02 Server status: All good\n2019/04/22 15:03:04 Server status: All good\n2019/04/22 15:03:06 Server status: All good\n2019/04/22 15:03:08 Server status: All good\n2019/04/22 15:03:10 Server status: All good\n2019/04/22 15:03:12 Server status: All good\n2019/04/22 15:03:14 Server status: All good\n2019/04/22 15:03:16 Server status: All good\n2019/04/22 15:03:18 Server status: All good\n2019/04/22 15:03:20 Server status: All good\n2019/04/22 15:03:22 Server status: All good\n2019/04/22 15:03:24 Server status: All good\n2019/04/22 15:03:26 Server status: All good\n2019/04/22 15:03:28 Server status: All good\n2019/04/22 15:03:30 Server status: All good\n2019/04/22 15:03:32 Server status: All good\n2019/04/22 15:03:34 Server status: All good\n2019/04/22 15:03:36 Server status: All good\n2019/04/22 15:03:38 Server status: All good\n2019/04/22 15:03:40 Server status: All good\n2019/04/22 15:03:42 Server status: All good\n2019/04/22 15:03:44 Server status: All good\n2019/04/22 15:03:46 Server status: All good\n2019/04/22 15:03:48 Server status: All good\n2019/04/22 15:03:50 Server status: All good\n2019/04/22 15:03:52 Server status: All good\n2019/04/22 15:03:54 Server status: All good\n2019/04/22 15:03:56 Server status: All good\n2019/04/22 15:03:58 Server status: All good\n2019/04/22 15:04:00 Server status: All good\n2019/04/22 15:04:02 Server status: All good\n2019/04/22 15:04:04 Server status: All good\n2019/04/22 15:04:06 Server status: All good\n2019/04/22 15:04:08 Server status: All good\n2019/04/22 15:04:10 Server status: All good\n2019/04/22 15:04:12 Server status: All good\n2019/04/22 15:04:14 Server status: All good\n2019/04/22 15:04:16 Server status: All good\n2019/04/22 15:04:18 Server status: All good\n2019/04/22 15:04:20 Server status: All good\n2019/04/22 15:04:22 Server status: All good\n2019/04/22 15:04:24 Server status: All good\n2019/04/22 15:04:26 Server status: All good\n2019/04/22 15:04:28 Server status: All good\n2019/04/22 15:04:30 Server status: All good\n2019/04/22 15:04:32 Server status: All good\n2019/04/22 15:04:34 Server status: All good\n2019/04/22 15:04:36 Server status: All good\n2019/04/22 15:04:38 Server status: All good\n2019/04/22 15:04:40 Server status: All good\n2019/04/22 15:04:42 Server status: All good\n2019/04/22 15:04:44 Server status: All good\n2019/04/22 15:04:46 Server status: All good\n2019/04/22 15:04:48 Server status: All good\n2019/04/22 15:04:50 Server status: All good\n2019/04/22 15:04:52 Server status: All good\n2019/04/22 15:04:54 Server status: All good\n2019/04/22 15:04:56 Server status: All good\n2019/04/22 15:04:58 Server status: All good\n2019/04/22 15:05:00 Server status: All good\n2019/04/22 15:05:02 Server status: All good\n2019/04/22 15:05:04 Server status: All good\n2019/04/22 15:05:06 Server status: All good\n2019/04/22 15:05:08 Server status: All good\n2019/04/22 15:05:10 Server status: All good\n2019/04/22 15:05:12 Server status: All good\n2019/04/22 15:05:14 Server status: All good\n2019/04/22 15:05:16 Server status: All good\n2019/04/22 15:05:18 Server status: All good\n2019/04/22 15:05:20 Server status: All good\n2019/04/22 15:05:22 Server status: All good\n2019/04/22 15:05:24 Server status: All good\n2019/04/22 15:05:26 Server status: All good\n2019/04/22 15:05:28 Server status: All good\n2019/04/22 15:05:30 Server status: All good\n2019/04/22 15:05:32 Server status: All good\n2019/04/22 15:05:34 Server status: All good\n2019/04/22 15:05:36 Server status: All good\n2019/04/22 15:05:38 Server status: All good\n2019/04/22 15:05:40 Server status: All good\n2019/04/22 15:05:42 Server status: All good\n2019/04/22 15:05:44 Server status: All good\n2019/04/22 15:05:46 Server status: All good\n2019/04/22 15:05:48 Server status: All good\n2019/04/22 15:05:50 Server status: All good\n2019/04/22 15:05:52 Server status: All good\n2019/04/22 15:05:54 Server status: All good\n2019/04/22 15:05:56 Server status: All good\n2019/04/22 15:05:58 Server status: All good\n2019/04/22 15:06:00 Server status: All good\n2019/04/22 15:06:02 Server status: All good\n2019/04/22 15:06:04 Server status: All good\n2019/04/22 15:06:06 Server status: All good\n2019/04/22 15:06:08 Server status: All good\n2019/04/22 15:06:10 Server status: All good\n2019/04/22 15:06:12 Server status: All good\n2019/04/22 15:06:14 Server status: All good\n2019/04/22 15:06:16 Server status: All good\n2019/04/22 15:06:18 Server status: All good\n2019/04/22 15:06:20 Server status: All good\n2019/04/22 15:06:22 Server status: All good\n2019/04/22 15:06:24 Server status: All good\n2019/04/22 15:06:26 Server status: All good\n2019/04/22 15:06:28 Server status: All good\n2019/04/22 15:06:30 Server status: All good\n2019/04/22 15:06:32 Server status: All good\n2019/04/22 15:06:34 Server status: All good\n2019/04/22 15:06:36 Server status: All good\n2019/04/22 15:06:38 Server status: All good\n2019/04/22 15:06:40 Server status: All good\n2019/04/22 15:06:42 Server status: All good\n2019/04/22 15:06:45 Server status: All good\n2019/04/22 15:06:47 Server status: All good\n2019/04/22 15:06:49 Server status: All good\n2019/04/22 15:06:51 Server status: All good\n2019/04/22 15:06:53 Server status: All good\n2019/04/22 15:06:55 Server status: All good\n2019/04/22 15:06:57 Server status: All good\n2019/04/22 15:06:59 Server status: All good\n2019/04/22 15:07:01 Server status: All good\n2019/04/22 15:07:03 Server status: All good\n2019/04/22 15:07:05 Server status: All good\n2019/04/22 15:07:07 Server status: All good\n2019/04/22 15:07:09 Server status: All good\n2019/04/22 15:07:11 Server status: All good\n2019/04/22 15:07:13 Server status: All good\n2019/04/22 15:07:15 Server status: All good\n2019/04/22 15:07:17 Server status: All good\n2019/04/22 15:07:19 Server status: All good\n2019/04/22 15:07:21 Server status: All good\n2019/04/22 15:07:23 Server status: All good\n2019/04/22 15:07:25 Server status: All good\n2019/04/22 15:07:27 Server status: All good\n2019/04/22 15:07:29 Server status: All good\n2019/04/22 15:07:31 Server status: All good\n2019/04/22 15:07:33 Server status: All good\n2019/04/22 15:07:35 Server status: All good\n2019/04/22 15:07:37 Server status: All good\n2019/04/22 15:07:39 Server status: All good\n2019/04/22 15:07:41 Server status: All good\n2019/04/22 15:07:43 Server status: All good\n2019/04/22 15:07:45 Server status: All good\n2019/04/22 15:07:47 Server status: All good\n2019/04/22 15:07:49 Server status: All good\n2019/04/22 15:07:51 Server status: All good\n2019/04/22 15:07:53 Server status: All good\n2019/04/22 15:07:55 Server status: All good\n2019/04/22 15:07:57 Server status: All good\n2019/04/22 15:07:59 Server status: All good\n2019/04/22 15:08:01 Server status: All good\n2019/04/22 15:08:03 Server status: All good\n2019/04/22 15:08:05 Server status: All good\n2019/04/22 15:08:07 Server status: All good\n2019/04/22 15:08:09 Server status: All good\n2019/04/22 15:08:11 Server status: All good\n2019/04/22 15:08:13 Server status: All good\n2019/04/22 15:08:15 Server status: All good\n2019/04/22 15:08:17 Server status: All good\n2019/04/22 15:08:19 Server status: All good\n2019/04/22 15:08:21 Server status: All good\n2019/04/22 15:08:23 Server status: All good\n2019/04/22 15:08:25 Server status: All good\n2019/04/22 15:08:27 Server status: All good\n2019/04/22 15:08:29 Server status: All good\n2019/04/22 15:08:31 Server status: All good\n2019/04/22 15:08:33 Server status: All good\n2019/04/22 15:08:35 Server status: All good\n2019/04/22 15:08:37 Server status: All good\n2019/04/22 15:08:39 Server status: All good\n2019/04/22 15:08:41 Server status: All good\n2019/04/22 15:08:43 Server status: All good\n2019/04/22 15:08:45 Server status: All good\n2019/04/22 15:08:47 Server status: All good\n2019/04/22 15:08:49 Server status: All good\n2019/04/22 15:08:51 Server status: All good\n2019/04/22 15:08:53 Server status: All good\n2019/04/22 15:08:55 Server status: All good\n2019/04/22 15:08:57 Server status: All good\n2019/04/22 15:08:59 Server status: All good\n2019/04/22 15:09:01 Server status: All good\n2019/04/22 15:09:03 Server status: All good\n2019/04/22 15:09:05 Server status: All good\n2019/04/22 15:09:07 Server status: All good\n2019/04/22 15:09:09 Server status: All good\n2019/04/22 15:09:11 Server status: All good\n2019/04/22 15:09:13 Server status: All good\n2019/04/22 15:09:15 Server status: All good\n2019/04/22 15:09:17 Server status: All good\n2019/04/22 15:09:19 Server status: All good\n2019/04/22 15:09:21 Server status: All good\n2019/04/22 15:09:23 Server status: All good\n2019/04/22 15:09:25 Server status: All good\n2019/04/22 15:09:27 Server status: All good\n2019/04/22 15:09:29 Server status: All good\n2019/04/22 15:09:31 Server status: All good\n2019/04/22 15:09:33 Server status: All good\n2019/04/22 15:09:35 Server status: All good\n2019/04/22 15:09:37 Server status: All good\n2019/04/22 15:09:39 Server status: All good\n2019/04/22 15:09:41 Server status: All good\n2019/04/22 15:09:43 Server status: All good\n2019/04/22 15:09:45 Server status: All good\n2019/04/22 15:09:47 Server status: All good\n2019/04/22 15:09:49 Server status: All good\n2019/04/22 15:09:51 Server status: All good\n2019/04/22 15:09:53 Server status: All good\n2019/04/22 15:09:55 Server status: All good\n2019/04/22 15:09:57 Server status: All good\n2019/04/22 15:09:59 Server status: All good\n2019/04/22 15:10:01 Server status: All good\n2019/04/22 15:10:03 Server status: All good\n2019/04/22 15:10:05 Server status: All good\n2019/04/22 15:10:07 Server status: All good\n2019/04/22 15:10:09 Server status: All good\n2019/04/22 15:10:11 Server status: All good\n2019/04/22 15:10:13 Server status: All good\n2019/04/22 15:10:15 Server status: All good\n2019/04/22 15:10:17 Server status: All good\n2019/04/22 15:10:19 Server status: All good\n2019/04/22 15:10:20 Server status: All good\n2019/04/22 15:10:22 Server status: All good\n2019/04/22 15:10:24 Server status: All good\n2019/04/22 15:10:26 Server status: All good\n2019/04/22 15:10:29 Server status: All good\n2019/04/22 15:10:31 Server status: All good\n2019/04/22 15:10:33 Server status: All good\n2019/04/22 15:10:35 Server status: All good\n2019/04/22 15:10:37 Server status: All good\n2019/04/22 15:10:39 Server status: All good\n2019/04/22 15:10:41 Server status: All good\n2019/04/22 15:10:43 Server status: All good\n2019/04/22 15:10:45 Server status: All good\n2019/04/22 15:10:47 Server status: All good\n2019/04/22 15:10:49 Server status: All good\n2019/04/22 15:10:50 Server status: All good\n2019/04/22 15:10:52 Server status: All good\n2019/04/22 15:10:54 Server status: All good\n2019/04/22 15:10:56 Server status: All good\n2019/04/22 15:10:58 Server status: All good\n2019/04/22 15:11:00 Server status: All good\n2019/04/22 15:11:02 Server status: All good\n2019/04/22 15:11:04 Server status: All good\n2019/04/22 15:11:06 Server status: All good\n2019/04/22 15:11:08 Server status: All good\n2019/04/22 15:11:10 Server status: All good\n2019/04/22 15:11:12 Server status: All good\n2019/04/22 15:11:14 Server status: All good\n2019/04/22 15:11:17 Server status: All good\n2019/04/22 15:11:19 Server status: All good\n2019/04/22 15:11:20 Server status: All good\n2019/04/22 15:11:22 Server status: All good\n2019/04/22 15:11:24 Server status: All good\n2019/04/22 15:11:26 Server status: All good\n2019/04/22 15:11:28 Server status: All good\n2019/04/22 15:11:30 Server status: All good\n2019/04/22 15:11:32 Server status: All good\n2019/04/22 15:11:34 Server status: All good\n2019/04/22 15:11:36 Server status: All good\n2019/04/22 15:11:38 Server status: All good\n2019/04/22 15:11:40 Server status: All good\n2019/04/22 15:11:42 Server status: All good\n2019/04/22 15:11:44 Server status: All good\n2019/04/22 15:11:46 Server status: All good\n2019/04/22 15:11:48 Server status: All good\n2019/04/22 15:11:50 Server status: All good\n2019/04/22 15:11:52 Server status: All good\n2019/04/22 15:11:54 Server status: All good\n2019/04/22 15:11:56 Server status: All good\n2019/04/22 15:11:58 Server status: All good\n2019/04/22 15:12:00 Server status: All good\n2019/04/22 15:12:02 Server status: All good\n2019/04/22 15:12:04 Server status: All good\n2019/04/22 15:12:06 Server status: All good\n2019/04/22 15:12:08 Server status: All good\n2019/04/22 15:12:10 Server status: All good\n2019/04/22 15:12:12 Server status: All good\n2019/04/22 15:12:14 Server status: All good\n2019/04/22 15:12:16 Server status: All good\n2019/04/22 15:12:18 Server status: All good\n2019/04/22 15:12:20 Server status: All good\n2019/04/22 15:12:22 Server status: All good\n2019/04/22 15:12:24 Server status: All good\n2019/04/22 15:12:26 Server status: All good\n2019/04/22 15:12:28 Server status: All good\n2019/04/22 15:12:30 Server status: All good\n2019/04/22 15:12:32 Server status: All good\n2019/04/22 15:12:34 Server status: All good\n2019/04/22 15:12:36 Server status: All good\n2019/04/22 15:12:38 Server status: All good\n2019/04/22 15:12:40 Server status: All good\n2019/04/22 15:12:42 Server status: All good\n2019/04/22 15:12:44 Server status: All good\n2019/04/22 15:12:46 Server status: All good\n2019/04/22 15:12:48 Server status: All good\n2019/04/22 15:12:50 Server status: All good\n2019/04/22 15:12:52 Server status: All good\n2019/04/22 15:12:54 Server status: All good\n2019/04/22 15:12:56 Server status: All good\n2019/04/22 15:12:58 Server status: All good\n2019/04/22 15:13:00 Server status: All good\n2019/04/22 15:13:02 Server status: All good\n2019/04/22 15:13:04 Server status: All good\n2019/04/22 15:13:06 Server status: All good\n2019/04/22 15:13:08 Server status: All good\n2019/04/22 15:13:10 Server status: All good\n2019/04/22 15:13:12 Server status: All good\n2019/04/22 15:13:14 Server status: All good\n2019/04/22 15:13:16 Server status: All good\n2019/04/22 15:13:18 Server status: All good\n2019/04/22 15:13:20 Server status: All good\n2019/04/22 15:13:22 Server status: All good\n2019/04/22 15:13:24 Server status: All good\n2019/04/22 15:13:26 Server status: All good\n2019/04/22 15:13:28 Server status: All good\n2019/04/22 15:13:30 Server status: All good\n2019/04/22 15:13:32 Server status: All good\n2019/04/22 15:13:34 Server status: All good\n2019/04/22 15:13:36 Server status: All good\n2019/04/22 15:13:38 Server status: All good\n2019/04/22 15:13:40 Server status: All good\n2019/04/22 15:13:42 Server status: All good\n2019/04/22 15:13:44 Server status: All good\n2019/04/22 15:13:46 Server status: All good\n2019/04/22 15:13:48 Server status: All good\n2019/04/22 15:13:50 Server status: All good\n2019/04/22 15:13:52 Server status: All good\n2019/04/22 15:13:54 Server status: All good\n2019/04/22 15:13:56 Server status: All good\n2019/04/22 15:13:58 Server status: All good\n2019/04/22 15:14:00 Server status: All good\n2019/04/22 15:14:02 Server status: All good\n2019/04/22 15:14:04 Server status: All good\n2019/04/22 15:14:06 Server status: All good\n2019/04/22 15:14:08 Server status: All good\n2019/04/22 15:14:10 Server status: All good\n2019/04/22 15:14:12 Server status: All good\n2019/04/22 15:14:14 Server status: All good\n2019/04/22 15:14:16 Server status: All good\n2019/04/22 15:14:18 Server status: All good\n2019/04/22 15:14:20 Server status: All good\n2019/04/22 15:14:22 Server status: All good\n2019/04/22 15:14:24 Server status: All good\n2019/04/22 15:14:26 Server status: All good\n2019/04/22 15:14:28 Server status: All good\n2019/04/22 15:14:30 Server status: All good\n2019/04/22 15:14:32 Server status: All good\n2019/04/22 15:14:34 Server status: All good\n2019/04/22 15:14:36 Server status: All good\n2019/04/22 15:14:38 Server status: All good\n2019/04/22 15:14:40 Server status: All good\n2019/04/22 15:14:42 Server status: All good\n2019/04/22 15:14:44 Server status: All good\n2019/04/22 15:14:46 Server status: All good\n2019/04/22 15:14:48 Server status: All good\n2019/04/22 15:14:50 Server status: All good\n2019/04/22 15:14:52 Server status: All good\n2019/04/22 15:14:54 Server status: All good\n2019/04/22 15:14:56 Server status: All good\n2019/04/22 15:14:58 Server status: All good\n2019/04/22 15:15:00 Server status: All good\n2019/04/22 15:15:02 Server status: All good\n2019/04/22 15:15:04 Server status: All good\n2019/04/22 15:15:06 Server status: All good\n2019/04/22 15:15:08 Server status: All good\n2019/04/22 15:15:10 Server status: All good\n2019/04/22 15:15:12 Server status: All good\n2019/04/22 15:15:14 Server status: All good\n2019/04/22 15:15:16 Server status: All good\n2019/04/22 15:15:18 Server status: All good\n2019/04/22 15:15:20 Server status: All good\n2019/04/22 15:15:22 Server status: All good\n2019/04/22 15:15:24 Server status: All good\n2019/04/22 15:15:26 Server status: All good\n2019/04/22 15:15:28 Server status: All good\n2019/04/22 15:15:30 Server status: All good\n2019/04/22 15:15:32 Server status: All good\n2019/04/22 15:15:34 Server status: All good\n2019/04/22 15:15:36 Server status: All good\n2019/04/22 15:15:38 Server status: All good\n2019/04/22 15:15:40 Server status: All good\n2019/04/22 15:15:42 Server status: All good\n2019/04/22 15:15:44 Server status: All good\n2019/04/22 15:15:46 Server status: All good\n2019/04/22 15:15:48 Server status: All good\n2019/04/22 15:15:50 Server status: All good\n2019/04/22 15:15:52 Server status: All good\n2019/04/22 15:15:54 Server status: All good\n2019/04/22 15:15:56 Server status: All good\n2019/04/22 15:15:58 Server status: All good\n2019/04/22 15:16:00 Server status: All good\n2019/04/22 15:16:02 Server status: All good\n2019/04/22 15:16:04 Server status: All good\n2019/04/22 15:16:06 Server status: All good\n2019/04/22 15:16:08 Server status: All good\n2019/04/22 15:16:10 Server status: All good\n2019/04/22 15:16:12 Server status: All good\n2019/04/22 15:16:14 Server status: All good\n2019/04/22 15:16:16 Server status: All good\n2019/04/22 15:16:18 Server status: All good\n2019/04/22 15:16:20 Server status: All good\n2019/04/22 15:16:22 Server status: All good\n2019/04/22 15:16:24 Server status: All good\n2019/04/22 15:16:26 Server status: All good\n2019/04/22 15:16:28 Server status: All good\n2019/04/22 15:16:30 Server status: All good\n2019/04/22 15:16:32 Server status: All good\n2019/04/22 15:16:34 Server status: All good\n2019/04/22 15:16:36 Server status: All good\n2019/04/22 15:16:38 Server status: All good\n2019/04/22 15:16:40 Server status: All good\n2019/04/22 15:16:42 Server status: All good\n2019/04/22 15:16:44 Server status: All good\n2019/04/22 15:16:46 Server status: All good\n2019/04/22 15:16:48 Server status: All good\n2019/04/22 15:16:50 Server status: All good\n2019/04/22 15:16:52 Server status: All good\n2019/04/22 15:16:54 Server status: All good\n2019/04/22 15:16:56 Server status: All good\n2019/04/22 15:16:58 Server status: All good\n2019/04/22 15:17:00 Server status: All good\n2019/04/22 15:17:02 Server status: All good\n2019/04/22 15:17:04 Server status: All good\n2019/04/22 15:17:06 Server status: All good\n2019/04/22 15:17:08 Server status: All good\n2019/04/22 15:17:10 Server status: All good\n2019/04/22 15:17:12 Server status: All good\n2019/04/22 15:17:14 Server status: All good\n2019/04/22 15:17:16 Server status: All good\n2019/04/22 15:17:18 Server status: All good\n2019/04/22 15:17:20 Server status: All good\n2019/04/22 15:17:22 Server status: All good\n2019/04/22 15:17:24 Server status: All good\n2019/04/22 15:17:26 Server status: All good\n2019/04/22 15:17:28 Server status: All good\n2019/04/22 15:17:30 Server status: All good\n2019/04/22 15:17:32 Server status: All good\n2019/04/22 15:17:34 Server status: All good\n2019/04/22 15:17:36 Server status: All good\n2019/04/22 15:17:38 Server status: All good\n2019/04/22 15:17:40 Server status: All good\n2019/04/22 15:17:42 Server status: All good\n2019/04/22 15:17:44 Server status: All good\n2019/04/22 15:17:46 Server status: All good\n2019/04/22 15:17:48 Server status: All good\n2019/04/22 15:17:50 Server status: All good\n2019/04/22 15:17:52 Server status: All good\n2019/04/22 15:17:54 Server status: All good\n2019/04/22 15:17:56 Server status: All good\n2019/04/22 15:17:58 Server status: All good\n2019/04/22 15:18:00 Server status: All good\n2019/04/22 15:18:02 Server status: All good\n2019/04/22 15:18:04 Server status: All good\n2019/04/22 15:18:06 Server status: All good\n2019/04/22 15:18:08 Server status: All good\n2019/04/22 15:18:10 Server status: All good\n2019/04/22 15:18:12 Server status: All good\n2019/04/22 15:18:14 Server status: All good\n2019/04/22 15:18:16 Server status: All good\n2019/04/22 15:18:18 Server status: All good\n2019/04/22 15:18:20 Server status: All good\n2019/04/22 15:18:22 Server status: All good\n2019/04/22 15:18:24 Server status: All good\n2019/04/22 15:18:26 Server status: All good\n2019/04/22 15:18:28 Server status: All good\n2019/04/22 15:18:30 Server status: All good\n2019/04/22 15:18:32 Server status: All good\n2019/04/22 15:18:34 Server status: All good\n2019/04/22 15:18:36 Server status: All good\n2019/04/22 15:18:38 Server status: All good\n2019/04/22 15:18:40 Server status: All good\n2019/04/22 15:18:42 Server status: All good\n2019/04/22 15:18:44 Server status: All good\n2019/04/22 15:18:46 Server status: All good\n2019/04/22 15:18:48 Server status: All good\n2019/04/22 15:18:50 Server status: All good\n2019/04/22 15:18:52 Server status: All good\n2019/04/22 15:18:54 Server status: All good\n2019/04/22 15:18:56 Server status: All good\n2019/04/22 15:18:58 Server status: All good\n2019/04/22 15:19:00 Server status: All good\n2019/04/22 15:19:02 Server status: All good\n2019/04/22 15:19:04 Server status: All good\n2019/04/22 15:19:06 Server status: All good\n2019/04/22 15:19:08 Server status: All good\n2019/04/22 15:19:10 Server status: All good\n2019/04/22 15:19:12 Server status: All good\n2019/04/22 15:19:14 Server status: All good\n2019/04/22 15:19:16 Server status: All good\n2019/04/22 15:19:18 Server status: All good\n2019/04/22 15:19:20 Server status: All good\n2019/04/22 15:19:22 Server status: All good\n2019/04/22 15:19:24 Server status: All good\n2019/04/22 15:19:26 Server status: All good\n2019/04/22 15:19:28 Server status: All good\n2019/04/22 15:19:30 Server status: All good\n2019/04/22 15:19:32 Server status: All good\n2019/04/22 15:19:34 Server status: All good\n2019/04/22 15:19:36 Server status: All good\n2019/04/22 15:19:38 Server status: All good\n2019/04/22 15:19:40 Server status: All good\n2019/04/22 15:19:42 Server status: All good\n2019/04/22 15:19:44 Server status: All good\n2019/04/22 15:19:46 Server status: All good\n2019/04/22 15:19:48 Server status: All good\n2019/04/22 15:19:50 Server status: All good\n2019/04/22 15:19:52 Server status: All good\n2019/04/22 15:19:54 Server status: All good\n2019/04/22 15:19:56 Server status: All good\n2019/04/22 15:19:58 Server status: All good\n2019/04/22 15:20:00 Server status: All good\n2019/04/22 15:20:02 Server status: All good\n2019/04/22 15:20:04 Server status: All good\n2019/04/22 15:20:06 Server status: All good\n2019/04/22 15:20:08 Server status: All good\n2019/04/22 15:20:10 Server status: All good\n2019/04/22 15:20:12 Server status: All good\n2019/04/22 15:20:14 Server status: All good\n2019/04/22 15:20:16 Server status: All good\n2019/04/22 15:20:18 Server status: All good\n2019/04/22 15:20:20 Server status: All good\n2019/04/22 15:20:22 Server status: All good\n2019/04/22 15:20:24 Server status: All good\n2019/04/22 15:20:26 Server status: All good\n2019/04/22 15:20:28 Server status: All good\n2019/04/22 15:20:30 Server status: All good\n2019/04/22 15:20:32 Server status: All good\n2019/04/22 15:20:34 Server status: All good\n2019/04/22 15:20:36 Server status: All good\n2019/04/22 15:20:38 Server status: All good\n2019/04/22 15:20:40 Server status: All good\n2019/04/22 15:20:42 Server status: All good\n2019/04/22 15:20:44 Server status: All good\n2019/04/22 15:20:46 Server status: All good\n",
    },
    {
      name: "snack",
      directoriesWatched: ["snack"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T11:05:58.928369-04:00",
      buildHistory: [
        {
          edits: ["main.go"],
          error: null,
          warnings: null,
          startTime: "2019-04-22T11:05:53.676776-04:00",
          finishTime: "2019-04-22T11:05:58.928367-04:00",
          log:
            "\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-41cf0bdf0c8d3aa7\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 4.271s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.980s\n\u001b[34m  │ \u001b[0mDone in: 5.250s \n\n",
        },
        {
          edits: ["main.go"],
          error: {},
          warnings: null,
          startTime: "2019-04-22T11:05:07.250689-04:00",
          finishTime: "2019-04-22T11:05:17.689819-04:00",
          log:
            "\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n    ╎   → # github.com/windmilleng/servantes/snack\nsrc/github.com/windmilleng/servantes/snack/main.go:21:17: syntax error: unexpected newline, expecting comma or }\n\n    ╎ ERROR IN: [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[31mERROR:\u001b[0m ImageBuild: executor failed running [/bin/sh -c go install github.com/windmilleng/servantes/snack]: exit code: 2\n",
        },
      ],
      currentBuild: {
        edits: ["main.go"],
        error: null,
        warnings: null,
        startTime: "2019-04-22T11:20:44.674248-04:00",
        finishTime: "0001-01-01T00:00:00Z",
        log:
          "\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n",
      },
      pendingBuildReason: 1,
      pendingBuildEdits: ["main.go"],
      pendingBuildSince: "2019-04-22T11:20:44.672903-04:00",
      endpoints: ["http://localhost:9002/"],
      k8sResourceInfo: {
        podName: "dan-snack-65f9775f49-gcc8d",
        podCreationTime: "2019-04-22T11:05:58-04:00",
        podUpdateStartTime: "2019-04-22T11:20:44.674248-04:00",
        podStatus: "CrashLoopBackOff",
        podRestarts: 7,
        podLog: "",
      },
      runtimeStatus: "error",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-13631d4ed09f1a05\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.241s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.190s\n\u001b[34m  │ \u001b[0mDone in: 1.431s \n\n2019/04/22 15:00:06 Starting Snack Service on :8083\n\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n    ╎   → # github.com/windmilleng/servantes/snack\nsrc/github.com/windmilleng/servantes/snack/main.go:21:17: syntax error: unexpected newline, expecting comma or }\n\n    ╎ ERROR IN: [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[31mERROR:\u001b[0m ImageBuild: executor failed running [/bin/sh -c go install github.com/windmilleng/servantes/snack]: exit code: 2\n\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-41cf0bdf0c8d3aa7\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 4.271s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.980s\n\u001b[34m  │ \u001b[0mDone in: 5.250s \n\n2019/04/22 15:06:00 Can't Find Necessary Resource File; dying\n2019/04/22 15:06:02 Can't Find Necessary Resource File; dying\n2019/04/22 15:06:16 Can't Find Necessary Resource File; dying\n2019/04/22 15:06:45 Can't Find Necessary Resource File; dying\n2019/04/22 15:07:28 Can't Find Necessary Resource File; dying\n2019/04/22 15:08:59 Can't Find Necessary Resource File; dying\n2019/04/22 15:11:52 Can't Find Necessary Resource File; dying\n2019/04/22 15:16:57 Can't Find Necessary Resource File; dying\n\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n",
    },
  ]
}

function oneResourceCrashedOnStart(): any {
  return [
    {
      name: "snack",
      directoriesWatched: ["snack"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-04-22T13:34:59.442147-04:00",
      buildHistory: [
        {
          edits: ["main.go"],
          error: null,
          warnings: null,
          startTime: "2019-04-22T13:34:57.084919-04:00",
          finishTime: "2019-04-22T13:34:59.442139-04:00",
          log:
            "\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-a2f42ad453eedd6d\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 2.134s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.222s\n\u001b[34m  │ \u001b[0mDone in: 2.356s \n\n",
        },
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-04-22T13:34:05.844691-04:00",
          finishTime: "2019-04-22T13:34:07.352812-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-44f988219ddc41f5\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.332s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.175s\n\u001b[34m  │ \u001b[0mDone in: 1.507s \n\n",
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpoints: ["http://localhost:9002/"],
      k8sResourceInfo: {
        podName: "dan-snack-cd4d74d7b-lg8sh",
        podCreationTime: "2019-04-22T13:34:59-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "CrashLoopBackOff",
        podRestarts: 1,
        podLog:
          "2019/04/22 17:35:02 Can't Find Necessary Resource File; dying\n",
      },
      runtimeStatus: "error",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-44f988219ddc41f5\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 1.332s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.175s\n\u001b[34m  │ \u001b[0mDone in: 1.507s \n\n2019/04/22 17:34:10 Starting Snack Service on :8083\n\n\u001b[32m1 changed: \u001b[0m[snack/main.go]\n\n\n\u001b[34m──┤ Rebuilding: \u001b[0msnack\u001b[34m ├────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 9.7 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-a2f42ad453eedd6d\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 2.134s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.222s\n\u001b[34m  │ \u001b[0mDone in: 2.356s \n\n2019/04/22 17:35:01 Can't Find Necessary Resource File; dying\n2019/04/22 17:35:02 Can't Find Necessary Resource File; dying\n",
      alerts: [],
    },
  ]
}

function oneResourceManualTriggerDirty(): any {
  return [
    {
      name: "(Tiltfile)",
      directoriesWatched: null,
      pathsWatched: null,
      lastDeployTime: "2019-06-12T12:33:27.831613-04:00",
      triggerMode: 0,
      buildHistory: [
        {
          edits: ["Tiltfile"],
          error: null,
          warnings: null,
          startTime: "2019-06-12T12:33:27.439018-04:00",
          finishTime: "2019-06-12T12:33:27.831613-04:00",
          log:
            'Beginning Tiltfile execution\nRunning `"whoami"`\nRunning `"m4 -Dvarowner=dan \\"deploy/fe.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/vigoda.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/snack.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/doggos.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/fortune.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hypothesizer.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/spoonerisms.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/emoji.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/words.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/secrets.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/job.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/sleeper.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hello_world.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/tick.yaml\\""`\nSuccessfully loaded Tiltfile\n',
          isCrashRebuild: false,
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
        isCrashRebuild: false,
      },
      pendingBuildReason: 0,
      pendingBuildEdits: null,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      hasPendingChanges: false,
      endpoints: null,
      podID: "",
      k8sResourceInfo: {
        podName: "",
        podCreationTime: "",
        podUpdateStartTime: "",
        podStatus: "",
        podStatusMessage: "",
        podRestarts: 0,
        podLog: "",
        endpoints: [],
      },
      runtimeStatus: "ok",
      isTiltfile: true,
      showBuildStatus: false,
      combinedLog:
        'Beginning Tiltfile execution\nRunning `"whoami"`\nRunning `"m4 -Dvarowner=dan \\"deploy/fe.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/vigoda.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/snack.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/doggos.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/fortune.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hypothesizer.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/spoonerisms.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/emoji.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/words.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/secrets.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/job.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/sleeper.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/hello_world.yaml\\""`\nRunning `"m4 -Dvarowner=dan \\"deploy/tick.yaml\\""`\nSuccessfully loaded Tiltfile\n',
      crashLog: "",
      alerts: [],
    },
    {
      name: "snack",
      directoriesWatched: ["snack"],
      pathsWatched: ["Tiltfile"],
      lastDeployTime: "2019-06-12T12:33:48.331048-04:00",
      triggerMode: 1,
      buildHistory: [
        {
          edits: null,
          error: null,
          warnings: null,
          startTime: "2019-06-12T12:33:42.848866-04:00",
          finishTime: "2019-06-12T12:33:48.331046-04:00",
          log:
            "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 10 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-fcf849b0f0bc9396\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 5.314s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.168s\n\u001b[34m  │ \u001b[0mDone in: 5.482s \n\n",
          isCrashRebuild: false,
        },
      ],
      currentBuild: {
        edits: null,
        error: null,
        warnings: null,
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
        log: "",
        isCrashRebuild: false,
      },
      pendingBuildReason: 1,
      pendingBuildEdits: ["hi"],
      pendingBuildSince: "2019-06-12T12:36:05.292424-04:00",
      hasPendingChanges: true,
      endpoints: null,
      podID: "dan-snack-85c688bffb-txf7z",
      k8sResourceInfo: {
        podName: "dan-snack-85c688bffb-txf7z",
        podCreationTime: "2019-06-12T12:33:48-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
        podLog: "2019/06/12 16:33:49 Starting Snack Service on :8083\n",
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
      combinedLog:
        "\n\u001b[34m──┤ Building: \u001b[0msnack\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/3 — \u001b[0mBuilding Dockerfile: [docker.io/library/snack]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/snack\n  RUN go install github.com/windmilleng/servantes/snack\n  \n  ENTRYPOINT /go/bin/snack\n\n\u001b[34m  │ \u001b[0mTarring context…\n    ╎ Created tarball (size: 10 kB)\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ [1/3] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/snack\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/snack\n\n\u001b[34mSTEP 2/3 — \u001b[0mPushing gcr.io/windmill-public-containers/servantes/snack:tilt-fcf849b0f0bc9396\n    ╎ Skipping push\n\n\u001b[34mSTEP 3/3 — \u001b[0mDeploying\n\u001b[34m  │ \u001b[0mParsing Kubernetes config YAML\n\u001b[34m  │ \u001b[0mApplying via kubectl\n\n\u001b[34m  │ \u001b[0mStep 1 - 5.314s\n\u001b[34m  │ \u001b[0mStep 2 - 0.000s\n\u001b[34m  │ \u001b[0mStep 3 - 0.168s\n\u001b[34m  │ \u001b[0mDone in: 5.482s \n\n2019/06/12 16:33:49 Starting Snack Service on :8083\n",
      crashLog: "",
      alerts: [],
    },
  ]
}

it("loads ok", () => {})
export {
  oneResource,
  oneResourceView,
  twoResourceView,
  getMockRouterProps,
  allResourcesOK,
  oneResourceFailedToBuild,
  oneResourceCrashedOnStart,
  oneResourceBuilding,
  oneResourceNoAlerts,
  oneResourceImagePullBackOff,
  oneResourceErrImgPull,
  oneResourceManualTriggerDirty,
  oneResourceUnrecognizedError,
}
