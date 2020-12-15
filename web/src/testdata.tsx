import { Href, UnregisterCallback } from "history"
import { RouteComponentProps } from "react-router-dom"
import { TriggerMode } from "./types"

type Resource = Proto.webviewResource
type Link = Proto.webviewLink

const unnamedEndpointLink: Link = { url: "1.2.3.4:8080" }
const namedEndpointLink: Link = { url: "1.2.3.4:9090", name: "debugger" }

type view = {
  resources: Array<Resource>
  logList?: Proto.webviewLogList
  featureFlags?: { [featureFlag: string]: boolean }
  tiltfileKey?: string
  runningTiltBuild?: Proto.webviewTiltBuild
}

let runningTiltBuild = {
  commitSHA: "658f2719f3380bee8e7119c7eb29f4a4a986ac6e",
  date: "2020-12-10",
  dev: true,
  version: "0.17.13",
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
      go: (num) => {},
      goBack: () => {},
      goForward: () => {},
      block: (t) => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
      createHref: (t) => {
        var temp: Href = ""
        return temp
      },
      listen: (t) => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
    },
    staticContext: {},
  }

  return props
}

function vigodaSpecs(): any {
  return [
    {
      hasLiveUpdate: false,
      id: "image:vigoda",
      type: "TARGET_TYPE_IMAGE",
    },
    {
      hasLiveUpdate: false,
      id: "k8s:vigoda",
      type: "TARGET_TYPE_K8S",
    },
  ]
}

export function tiltfileResource(): Resource {
  const ts = new Date(Date.now()).toISOString()
  const tsPast = new Date(Date.now() - 12300).toISOString()
  const resource: Resource = {
    name: "(Tiltfile)",
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["Tiltfile"],
        finishTime: ts,
        startTime: tsPast,
      },
    ],
    runtimeStatus: "not_applicable",
    crashLog: "",
    triggerMode: TriggerMode.TriggerModeAuto,
    hasPendingChanges: false,
    endpointLinks: [],
    podID: "",
    isTiltfile: true,
    facets: [],
    queued: false,
  }
  return resource
}

function oneResource(): Resource {
  const ts = new Date(Date.now()).toISOString()
  const tsPast = new Date(Date.now() - 12300).toISOString()
  const resource: Resource = {
    name: "vigoda",
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: "the build failed!",
        finishTime: ts,
        startTime: tsPast,
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
      podUpdateStartTime: ts,
    },
    runtimeStatus: "ok",
    crashLog: "",
    triggerMode: TriggerMode.TriggerModeAuto,
    hasPendingChanges: false,
    endpointLinks: [],
    podID: "",
    isTiltfile: false,
    facets: [],
    queued: false,
    specs: vigodaSpecs(),
  }
  return resource
}

function oneResourceNoAlerts(): Resource {
  const ts = new Date(Date.now()).toISOString()
  const resource = {
    name: "vigoda",
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
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
    },
    endpointLinks: [unnamedEndpointLink],
    runtimeStatus: "ok",
    specs: vigodaSpecs(),
  }
  return resource
}

function oneResourceImagePullBackOff(): Resource {
  const ts = new Date(Date.now()).toISOString()
  const resource = {
    name: "vigoda",
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
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
    },
    endpointLinks: [unnamedEndpointLink],
    runtimeStatus: "ok",
    specs: vigodaSpecs(),
  }
  return resource
}

function oneResourceErrImgPull(): Resource {
  const ts = new Date(Date.now()).toISOString()
  const resource = {
    name: "vigoda",
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
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
    },
    endpointLinks: [unnamedEndpointLink],
    runtimeStatus: "ok",
    specs: vigodaSpecs(),
  }
  return resource
}

function oneResourceUnrecognizedError(): Resource {
  const ts = new Date(Date.now()).toISOString()
  const resource = {
    name: "vigoda",
    lastDeployTime: ts,
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
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
    specs: vigodaSpecs(),
  }
  return resource
}

function oneResourceView(): view {
  return { resources: [oneResource()], tiltfileKey: "test", runningTiltBuild }
}

function twoResourceView(): view {
  const time = Date.now()
  const ts = new Date(time).toISOString()
  const vigoda = oneResource()

  const snack: Resource = {
    name: "snack",
    lastDeployTime: new Date(time - 10000).toISOString(),
    buildHistory: [
      {
        edits: ["main.go", "cli.go"],
        error: "the build failed!",
        finishTime: new Date(time - 10000).toISOString(),
        startTime: ts,
      },
    ],
    pendingBuildEdits: ["main.go", "cli.go", "snack.go"],
    pendingBuildSince: ts,
    currentBuild: {
      edits: ["main.go"],
      startTime: ts,
    },
    endpointLinks: [unnamedEndpointLink],
    runtimeStatus: "ok",
    triggerMode: TriggerMode.TriggerModeAuto,
    crashLog: "",
    isTiltfile: false,
    podID: "",
    pendingBuildReason: 0,
    k8sResourceInfo: {
      podStatus: "Running",
      podStatusMessage: "",
      podRestarts: 0,
      podCreationTime: "",
      podName: "snack",
      podUpdateStartTime: "",
    },
    hasPendingChanges: false,
    facets: [],
    queued: false,
    specs: vigodaSpecs(),
  }
  return { resources: [vigoda, snack], tiltfileKey: "test", runningTiltBuild }
}

export function tenResourceView(): view {
  return nResourceView(10)
}

export function nResourceView(n: number): view {
  let resources: Resource[] = []
  for (let i = 0; i < n; i++) {
    if (i === 0) {
      let res = tiltfileResource()
      resources.push(res)
    } else {
      let res = oneResourceNoAlerts()
      res.name += "_" + i
      resources.push(res)
    }
  }
  return { resources: resources, tiltfileKey: "test", runningTiltBuild }
}

function oneResourceFailedToBuild(): Resource[] {
  return [
    {
      name: "snack",
      lastDeployTime: "2019-04-22T11:00:04.242586-04:00",
      buildHistory: [
        {
          edits: ["main.go"],
          error: "oh no",
          startTime: "2019-04-22T11:05:07.250689-04:00",
          finishTime: "2019-04-22T11:05:17.689819-04:00",
        },
        {
          startTime: "2019-04-22T11:00:02.810268-04:00",
          finishTime: "2019-04-22T11:00:04.242583-04:00",
        },
      ],
      currentBuild: {
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
      },
      pendingBuildReason: 1,
      pendingBuildEdits: ["main.go"],
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpointLinks: [{ url: "http://localhost:9002/" }],
      k8sResourceInfo: {
        podName: "dan-snack-f885fb46f-d5z2t",
        podCreationTime: "2019-04-22T11:00:04-04:00",
        podUpdateStartTime: "2019-04-22T11:05:07.250689-04:00",
        podStatus: "Running",
        podRestarts: 0,
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
    },
  ]
}

function oneResourceBuilding(): Resource[] {
  return [
    {
      name: "(Tiltfile)",
      lastDeployTime: "2019-04-22T10:59:53.903047-04:00",
      buildHistory: [
        {
          edits: ["/Users/dan/go/src/github.com/tilt-dev/servantes/Tiltfile"],
          startTime: "2019-04-22T10:59:53.574652-04:00",
          finishTime: "2019-04-22T10:59:53.903047-04:00",
        },
      ],
      currentBuild: {
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
      },
      pendingBuildReason: 0,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      runtimeStatus: "ok",
      isTiltfile: true,
      showBuildStatus: false,
    },
    {
      name: "fe",
      lastDeployTime: "2019-04-22T11:00:01.337285-04:00",
      buildHistory: [
        {
          startTime: "2019-04-22T10:59:56.489417-04:00",
          finishTime: "2019-04-22T11:00:01.337284-04:00",
        },
      ],
      currentBuild: {
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
      },
      pendingBuildReason: 0,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpointLinks: [{ url: "http://localhost:9000/" }],
      k8sResourceInfo: {
        podName: "dan-fe-7cdc8f978f-vp94d",
        podCreationTime: "2019-04-22T11:00:01-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
    },
    {
      name: "vigoda",
      lastDeployTime: "2019-04-22T11:00:02.810113-04:00",
      buildHistory: [
        {
          startTime: "2019-04-22T11:00:01.337359-04:00",
          finishTime: "2019-04-22T11:00:02.810112-04:00",
        },
      ],
      currentBuild: {
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
      },
      pendingBuildReason: 0,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpointLinks: [{ url: "http://localhost:9001/" }],
      k8sResourceInfo: {
        podName: "dan-vigoda-67d79bd8d5-w77q4",
        podCreationTime: "2019-04-22T11:00:02-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "Running",
        podRestarts: 0,
      },
      runtimeStatus: "ok",
      isTiltfile: false,
      showBuildStatus: true,
    },
    {
      name: "snack",
      lastDeployTime: "2019-04-22T11:05:58.928369-04:00",
      buildHistory: [
        {
          edits: ["main.go"],
          startTime: "2019-04-22T11:05:53.676776-04:00",
          finishTime: "2019-04-22T11:05:58.928367-04:00",
        },
        {
          edits: ["main.go"],
          error: "eek",
          startTime: "2019-04-22T11:05:07.250689-04:00",
          finishTime: "2019-04-22T11:05:17.689819-04:00",
        },
      ],
      currentBuild: {
        edits: ["main.go"],
        startTime: "2019-04-22T11:20:44.674248-04:00",
        finishTime: "0001-01-01T00:00:00Z",
      },
      pendingBuildReason: 1,
      pendingBuildEdits: ["main.go"],
      pendingBuildSince: "2019-04-22T11:20:44.672903-04:00",
      endpointLinks: [{ url: "http://localhost:9002/" }],
      k8sResourceInfo: {
        podName: "dan-snack-65f9775f49-gcc8d",
        podCreationTime: "2019-04-22T11:05:58-04:00",
        podUpdateStartTime: "2019-04-22T11:20:44.674248-04:00",
        podStatus: "CrashLoopBackOff",
        podRestarts: 7,
      },
      runtimeStatus: "error",
      isTiltfile: false,
      showBuildStatus: true,
    },
  ]
}

function oneResourceCrashedOnStart(): Resource[] {
  return [
    {
      name: "snack",
      lastDeployTime: "2019-04-22T13:34:59.442147-04:00",
      buildHistory: [
        {
          edits: ["main.go"],
          startTime: "2019-04-22T13:34:57.084919-04:00",
          finishTime: "2019-04-22T13:34:59.442139-04:00",
        },
        {
          startTime: "2019-04-22T13:34:05.844691-04:00",
          finishTime: "2019-04-22T13:34:07.352812-04:00",
        },
      ],
      currentBuild: {
        startTime: "0001-01-01T00:00:00Z",
        finishTime: "0001-01-01T00:00:00Z",
      },
      pendingBuildReason: 0,
      pendingBuildSince: "0001-01-01T00:00:00Z",
      endpointLinks: [{ url: "http://localhost:9002/" }],
      k8sResourceInfo: {
        podName: "dan-snack-cd4d74d7b-lg8sh",
        podCreationTime: "2019-04-22T13:34:59-04:00",
        podUpdateStartTime: "0001-01-01T00:00:00Z",
        podStatus: "CrashLoopBackOff",
        podRestarts: 1,
      },
      runtimeStatus: "error",
      isTiltfile: false,
      showBuildStatus: true,
    },
  ]
}

const logPaneDOM = `<section class="LogPane"><span data-lineid="0" class="logLine "><code><span class="ansi-green">Starting Tilt (v0.10.18-dev, built 2019-11-13)…</span></code>
<br>
</span><span data-lineid="1" class="logLine "><code><span>Beginning Tiltfile execution</span></code>
<br>
</span><span data-lineid="2" class="logLine "><code><span>local: whoami</span></code>
<br>
</span><span data-lineid="3" class="logLine "><code><span>Installing Tilt NodeJS dependencies…</span></code>
<br>
</span><span data-lineid="4" class="logLine "><code><span> → dan</span></code>
<br>
</span><span data-lineid="5" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/fe.yaml"</span></code>
<br>
</span><span data-lineid="6" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="7" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="8" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="9" class="logLine "><code><span> →   name: dan-fe</span></code>
<br>
</span><span data-lineid="10" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="11" class="logLine "><code><span> →     app: fe</span></code>
<br>
</span><span data-lineid="12" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="13" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="14" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="15" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="16" class="logLine "><code><span> →       app: fe</span></code>
<br>
</span><span data-lineid="17" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="18" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="19" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="20" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="21" class="logLine "><code><span> →         app: fe</span></code>
<br>
</span><span data-lineid="22" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="23" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="24" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="25" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="26" class="logLine "><code><span> →       - name: fe</span></code>
<br>
</span><span data-lineid="27" class="logLine "><code><span> →         image: fe</span></code>
<br>
</span><span data-lineid="28" class="logLine "><code><span> →         env:</span></code>
<br>
</span><span data-lineid="29" class="logLine "><code><span> →         - name: TEMPLATE_DIR</span></code>
<br>
</span><span data-lineid="30" class="logLine "><code><span> →           value: "/go/src/github.com/tilt-dev/servantes/fe/web/templates"</span></code>
<br>
</span><span data-lineid="31" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="32" class="logLine "><code><span> →         - containerPort: 8080</span></code>
<br>
</span><span data-lineid="33" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="34" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="35" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="36" class="logLine "><code><span> → ---</span></code>
<br>
</span><span data-lineid="37" class="logLine "><code><span> → apiVersion: v1</span></code>
<br>
</span><span data-lineid="38" class="logLine "><code><span> → kind: Service</span></code>
<br>
</span><span data-lineid="39" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="40" class="logLine "><code><span> →   name: dan-fe</span></code>
<br>
</span><span data-lineid="41" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="42" class="logLine "><code><span> →     app: fe</span></code>
<br>
</span><span data-lineid="43" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="44" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="45" class="logLine "><code><span> →   type: LoadBalancer</span></code>
<br>
</span><span data-lineid="46" class="logLine "><code><span> →   ports:</span></code>
<br>
</span><span data-lineid="47" class="logLine "><code><span> →     - port: 8080</span></code>
<br>
</span><span data-lineid="48" class="logLine "><code><span> →       targetPort: 8080</span></code>
<br>
</span><span data-lineid="49" class="logLine "><code><span> →       protocol: TCP</span></code>
<br>
</span><span data-lineid="50" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="51" class="logLine "><code><span> →     app: fe</span></code>
<br>
</span><span data-lineid="52" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="53" class="logLine "><code><span> → ---</span></code>
<br>
</span><span data-lineid="54" class="logLine "><code><span> → kind: Role</span></code>
<br>
</span><span data-lineid="55" class="logLine "><code><span> → apiVersion: rbac.authorization.k8s.io/v1</span></code>
<br>
</span><span data-lineid="56" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="57" class="logLine "><code><span> →   name: pod-reader</span></code>
<br>
</span><span data-lineid="58" class="logLine "><code><span> → rules:</span></code>
<br>
</span><span data-lineid="59" class="logLine "><code><span> → - apiGroups: [""] # "" indicates the core API group</span></code>
<br>
</span><span data-lineid="60" class="logLine "><code><span> →   resources: ["pods"]</span></code>
<br>
</span><span data-lineid="61" class="logLine "><code><span> →   verbs: ["get", "watch", "list"]</span></code>
<br>
</span><span data-lineid="62" class="logLine "><code><span> → ---</span></code>
<br>
</span><span data-lineid="63" class="logLine "><code><span> → kind: RoleBinding</span></code>
<br>
</span><span data-lineid="64" class="logLine "><code><span> → apiVersion: rbac.authorization.k8s.io/v1</span></code>
<br>
</span><span data-lineid="65" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="66" class="logLine "><code><span> →   name: read-pods</span></code>
<br>
</span><span data-lineid="67" class="logLine "><code><span> → subjects:</span></code>
<br>
</span><span data-lineid="68" class="logLine "><code><span> → - kind: User</span></code>
<br>
</span><span data-lineid="69" class="logLine "><code><span> →   name: system:serviceaccount:default:default</span></code>
<br>
</span><span data-lineid="70" class="logLine "><code><span> → roleRef:</span></code>
<br>
</span><span data-lineid="71" class="logLine "><code><span> →   kind: Role</span></code>
<br>
</span><span data-lineid="72" class="logLine "><code><span> →   name: pod-reader</span></code>
<br>
</span><span data-lineid="73" class="logLine "><code><span> →   apiGroup: rbac.authorization.k8s.io</span></code>
<br>
</span><span data-lineid="74" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/vigoda.yaml"</span></code>
<br>
</span><span data-lineid="75" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="76" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="77" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="78" class="logLine "><code><span> →   name: dan-vigoda</span></code>
<br>
</span><span data-lineid="79" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="80" class="logLine "><code><span> →     app: vigoda</span></code>
<br>
</span><span data-lineid="81" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="82" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="83" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="84" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="85" class="logLine "><code><span> →       app: vigoda</span></code>
<br>
</span><span data-lineid="86" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="87" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="88" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="89" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="90" class="logLine "><code><span> →         app: vigoda</span></code>
<br>
</span><span data-lineid="91" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="92" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="93" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="94" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="95" class="logLine "><code><span> →       - name: vigoda</span></code>
<br>
</span><span data-lineid="96" class="logLine "><code><span> →         image: vigoda</span></code>
<br>
</span><span data-lineid="97" class="logLine "><code><span> →         command: ["/go/bin/vigoda"]</span></code>
<br>
</span><span data-lineid="98" class="logLine "><code><span> →         env:</span></code>
<br>
</span><span data-lineid="99" class="logLine "><code><span> →         - name: TEMPLATE_DIR</span></code>
<br>
</span><span data-lineid="100" class="logLine "><code><span> →           value: "/go/src/github.com/tilt-dev/servantes/vigoda/web/templates"</span></code>
<br>
</span><span data-lineid="101" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="102" class="logLine "><code><span> →         - containerPort: 8081</span></code>
<br>
</span><span data-lineid="103" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="104" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="105" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="106" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/snack.yaml"</span></code>
<br>
</span><span data-lineid="107" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="108" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="109" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="110" class="logLine "><code><span> →   name: dan-snack</span></code>
<br>
</span><span data-lineid="111" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="112" class="logLine "><code><span> →     app: snack</span></code>
<br>
</span><span data-lineid="113" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="114" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="115" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="116" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="117" class="logLine "><code><span> →       app: snack</span></code>
<br>
</span><span data-lineid="118" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="119" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="120" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="121" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="122" class="logLine "><code><span> →         app: snack</span></code>
<br>
</span><span data-lineid="123" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="124" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="125" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="126" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="127" class="logLine "><code><span> →       - name: snack</span></code>
<br>
</span><span data-lineid="128" class="logLine "><code><span> →         image: snack</span></code>
<br>
</span><span data-lineid="129" class="logLine "><code><span> →         env:</span></code>
<br>
</span><span data-lineid="130" class="logLine "><code><span> →         - name: TEMPLATE_DIR</span></code>
<br>
</span><span data-lineid="131" class="logLine "><code><span> →           value: "/go/src/github.com/tilt-dev/servantes/snack/web/templates"</span></code>
<br>
</span><span data-lineid="132" class="logLine "><code><span> →         - name: OWNER</span></code>
<br>
</span><span data-lineid="133" class="logLine "><code><span> →           value: dan</span></code>
<br>
</span><span data-lineid="134" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="135" class="logLine "><code><span> →         - containerPort: 8083</span></code>
<br>
</span><span data-lineid="136" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="137" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="138" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="139" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/doggos.yaml"</span></code>
<br>
</span><span data-lineid="140" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="141" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="142" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="143" class="logLine "><code><span> →   name: dan-doggos</span></code>
<br>
</span><span data-lineid="144" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="145" class="logLine "><code><span> →     app: doggos</span></code>
<br>
</span><span data-lineid="146" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="147" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="148" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="149" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="150" class="logLine "><code><span> →       app: doggos</span></code>
<br>
</span><span data-lineid="151" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="152" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="153" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="154" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="155" class="logLine "><code><span> →         app: doggos</span></code>
<br>
</span><span data-lineid="156" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="157" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="158" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="159" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="160" class="logLine "><code><span> →       - name: doggos</span></code>
<br>
</span><span data-lineid="161" class="logLine "><code><span> →         image: doggos</span></code>
<br>
</span><span data-lineid="162" class="logLine "><code><span> →         command: ["/go/bin/doggos"]</span></code>
<br>
</span><span data-lineid="163" class="logLine "><code><span> →         env:</span></code>
<br>
</span><span data-lineid="164" class="logLine "><code><span> →         - name: TEMPLATE_DIR</span></code>
<br>
</span><span data-lineid="165" class="logLine "><code><span> →           value: "/go/src/github.com/tilt-dev/servantes/doggos/web/templates"</span></code>
<br>
</span><span data-lineid="166" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="167" class="logLine "><code><span> →         - containerPort: 8083</span></code>
<br>
</span><span data-lineid="168" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="169" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="170" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="171" class="logLine "><code><span> →       - name: sidecar</span></code>
<br>
</span><span data-lineid="172" class="logLine "><code><span> →         image: sidecar</span></code>
<br>
</span><span data-lineid="173" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="174" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="175" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="176" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/fortune.yaml"</span></code>
<br>
</span><span data-lineid="177" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="178" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="179" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="180" class="logLine "><code><span> →   name: dan-fortune</span></code>
<br>
</span><span data-lineid="181" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="182" class="logLine "><code><span> →     app: fortune</span></code>
<br>
</span><span data-lineid="183" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="184" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="185" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="186" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="187" class="logLine "><code><span> →       app: fortune</span></code>
<br>
</span><span data-lineid="188" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="189" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="190" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="191" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="192" class="logLine "><code><span> →         app: fortune</span></code>
<br>
</span><span data-lineid="193" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="194" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="195" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="196" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="197" class="logLine "><code><span> →       - name: fortune</span></code>
<br>
</span><span data-lineid="198" class="logLine "><code><span> →         image: fortune</span></code>
<br>
</span><span data-lineid="199" class="logLine "><code><span> →         command: ["/go/bin/fortune"]</span></code>
<br>
</span><span data-lineid="200" class="logLine "><code><span> →         env:</span></code>
<br>
</span><span data-lineid="201" class="logLine "><code><span> →         - name: TEMPLATE_DIR</span></code>
<br>
</span><span data-lineid="202" class="logLine "><code><span> →           value: "/go/src/github.com/tilt-dev/servantes/fortune/web/templates"</span></code>
<br>
</span><span data-lineid="203" class="logLine "><code><span> →         - name: THE_SECRET</span></code>
<br>
</span><span data-lineid="204" class="logLine "><code><span> →           valueFrom:</span></code>
<br>
</span><span data-lineid="205" class="logLine "><code><span> →             secretKeyRef:</span></code>
<br>
</span><span data-lineid="206" class="logLine "><code><span> →               name: dan-servantes-stuff</span></code>
<br>
</span><span data-lineid="207" class="logLine "><code><span> →               key: things</span></code>
<br>
</span><span data-lineid="208" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="209" class="logLine "><code><span> →         - containerPort: 8082</span></code>
<br>
</span><span data-lineid="210" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="211" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="212" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="213" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/hypothesizer.yaml"</span></code>
<br>
</span><span data-lineid="214" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="215" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="216" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="217" class="logLine "><code><span> →   name: dan-hypothesizer</span></code>
<br>
</span><span data-lineid="218" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="219" class="logLine "><code><span> →     app: hypothesizer</span></code>
<br>
</span><span data-lineid="220" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="221" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="222" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="223" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="224" class="logLine "><code><span> →       app: hypothesizer</span></code>
<br>
</span><span data-lineid="225" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="226" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="227" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="228" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="229" class="logLine "><code><span> →         app: hypothesizer</span></code>
<br>
</span><span data-lineid="230" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="231" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="232" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="233" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="234" class="logLine "><code><span> →       - name: hypothesizer</span></code>
<br>
</span><span data-lineid="235" class="logLine "><code><span> →         image: hypothesizer</span></code>
<br>
</span><span data-lineid="236" class="logLine "><code><span> →         command: ["python", "/app/app.py"]</span></code>
<br>
</span><span data-lineid="237" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="238" class="logLine "><code><span> →         - containerPort: 5000</span></code>
<br>
</span><span data-lineid="239" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="240" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="241" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="242" class="logLine "><code><span> → ---</span></code>
<br>
</span><span data-lineid="243" class="logLine "><code><span> → apiVersion: v1</span></code>
<br>
</span><span data-lineid="244" class="logLine "><code><span> → kind: Service</span></code>
<br>
</span><span data-lineid="245" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="246" class="logLine "><code><span> →   name: dan-hypothesizer</span></code>
<br>
</span><span data-lineid="247" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="248" class="logLine "><code><span> →     app: hypothesizer</span></code>
<br>
</span><span data-lineid="249" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="250" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="251" class="logLine "><code><span> →   ports:</span></code>
<br>
</span><span data-lineid="252" class="logLine "><code><span> →     - port: 80</span></code>
<br>
</span><span data-lineid="253" class="logLine "><code><span> →       targetPort: 5000</span></code>
<br>
</span><span data-lineid="254" class="logLine "><code><span> →       protocol: TCP</span></code>
<br>
</span><span data-lineid="255" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="256" class="logLine "><code><span> →     app: hypothesizer</span></code>
<br>
</span><span data-lineid="257" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="258" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/spoonerisms.yaml"</span></code>
<br>
</span><span data-lineid="259" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="260" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="261" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="262" class="logLine "><code><span> →   name: dan-spoonerisms</span></code>
<br>
</span><span data-lineid="263" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="264" class="logLine "><code><span> →     app: spoonerisms</span></code>
<br>
</span><span data-lineid="265" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="266" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="267" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="268" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="269" class="logLine "><code><span> →       app: spoonerisms</span></code>
<br>
</span><span data-lineid="270" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="271" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="272" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="273" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="274" class="logLine "><code><span> →         app: spoonerisms</span></code>
<br>
</span><span data-lineid="275" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="276" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="277" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="278" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="279" class="logLine "><code><span> →       - name: spoonerisms</span></code>
<br>
</span><span data-lineid="280" class="logLine "><code><span> →         image: spoonerisms</span></code>
<br>
</span><span data-lineid="281" class="logLine "><code><span> →         command: ["node", "/app/index.js"]</span></code>
<br>
</span><span data-lineid="282" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="283" class="logLine "><code><span> →         - containerPort: 5000</span></code>
<br>
</span><span data-lineid="284" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="285" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="286" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="287" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/emoji.yaml"</span></code>
<br>
</span><span data-lineid="288" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="289" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="290" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="291" class="logLine "><code><span> →   name: dan-emoji</span></code>
<br>
</span><span data-lineid="292" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="293" class="logLine "><code><span> →     app: emoji</span></code>
<br>
</span><span data-lineid="294" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="295" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="296" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="297" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="298" class="logLine "><code><span> →       app: emoji</span></code>
<br>
</span><span data-lineid="299" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="300" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="301" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="302" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="303" class="logLine "><code><span> →         app: emoji</span></code>
<br>
</span><span data-lineid="304" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="305" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="306" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="307" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="308" class="logLine "><code><span> →       - name: emoji</span></code>
<br>
</span><span data-lineid="309" class="logLine "><code><span> →         image: emoji</span></code>
<br>
</span><span data-lineid="310" class="logLine "><code><span> →         command: ["/go/bin/emoji"]</span></code>
<br>
</span><span data-lineid="311" class="logLine "><code><span> →         env:</span></code>
<br>
</span><span data-lineid="312" class="logLine "><code><span> →         - name: TEMPLATE_DIR</span></code>
<br>
</span><span data-lineid="313" class="logLine "><code><span> →           value: "/go/src/github.com/tilt-dev/servantes/emoji/web/templates"</span></code>
<br>
</span><span data-lineid="314" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="315" class="logLine "><code><span> →         - containerPort: 8081</span></code>
<br>
</span><span data-lineid="316" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="317" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="318" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="319" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/words.yaml"</span></code>
<br>
</span><span data-lineid="320" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="321" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="322" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="323" class="logLine "><code><span> →   name: dan-words</span></code>
<br>
</span><span data-lineid="324" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="325" class="logLine "><code><span> →     app: words</span></code>
<br>
</span><span data-lineid="326" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="327" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="328" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="329" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="330" class="logLine "><code><span> →       app: words</span></code>
<br>
</span><span data-lineid="331" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="332" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="333" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="334" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="335" class="logLine "><code><span> →         app: words</span></code>
<br>
</span><span data-lineid="336" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="337" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="338" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="339" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="340" class="logLine "><code><span> →       - name: words</span></code>
<br>
</span><span data-lineid="341" class="logLine "><code><span> →         image: words</span></code>
<br>
</span><span data-lineid="342" class="logLine "><code><span> →         command: ["python", "/app/app.py"]</span></code>
<br>
</span><span data-lineid="343" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="344" class="logLine "><code><span> →         - containerPort: 5000</span></code>
<br>
</span><span data-lineid="345" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="346" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="347" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="348" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/random.yaml"</span></code>
<br>
</span><span data-lineid="349" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="350" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="351" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="352" class="logLine "><code><span> →   name: dan-random</span></code>
<br>
</span><span data-lineid="353" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="354" class="logLine "><code><span> →     app: random</span></code>
<br>
</span><span data-lineid="355" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="356" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="357" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="358" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="359" class="logLine "><code><span> →       app: random</span></code>
<br>
</span><span data-lineid="360" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="361" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="362" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="363" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="364" class="logLine "><code><span> →         app: random</span></code>
<br>
</span><span data-lineid="365" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="366" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="367" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="368" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="369" class="logLine "><code><span> →       - name: random</span></code>
<br>
</span><span data-lineid="370" class="logLine "><code><span> →         image: random</span></code>
<br>
</span><span data-lineid="371" class="logLine "><code><span> →         command: ["/go/bin/random"]</span></code>
<br>
</span><span data-lineid="372" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="373" class="logLine "><code><span> →         - containerPort: 8083</span></code>
<br>
</span><span data-lineid="374" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="375" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="376" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="377" class="logLine "><code><span> → </span></code>
<br>
</span><span data-lineid="378" class="logLine "><code><span> → </span></code>
<br>
</span><span data-lineid="379" class="logLine "><code><span> → ---</span></code>
<br>
</span><span data-lineid="380" class="logLine "><code><span> → apiVersion: v1</span></code>
<br>
</span><span data-lineid="381" class="logLine "><code><span> → kind: Service</span></code>
<br>
</span><span data-lineid="382" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="383" class="logLine "><code><span> →   name: dan-random</span></code>
<br>
</span><span data-lineid="384" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="385" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="386" class="logLine "><code><span> →     app: random</span></code>
<br>
</span><span data-lineid="387" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="388" class="logLine "><code><span> →   ports:</span></code>
<br>
</span><span data-lineid="389" class="logLine "><code><span> →     - protocol: TCP</span></code>
<br>
</span><span data-lineid="390" class="logLine "><code><span> →       port: 80</span></code>
<br>
</span><span data-lineid="391" class="logLine "><code><span> →       targetPort: 8083local: m4 -Dvarowner=dan "deploy/secrets.yaml"</span></code>
<br>
</span><span data-lineid="392" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="393" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="394" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="395" class="logLine "><code><span> →   name: dan-secrets</span></code>
<br>
</span><span data-lineid="396" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="397" class="logLine "><code><span> →     app: secrets</span></code>
<br>
</span><span data-lineid="398" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="399" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="400" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="401" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="402" class="logLine "><code><span> →       app: secrets</span></code>
<br>
</span><span data-lineid="403" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="404" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="405" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="406" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="407" class="logLine "><code><span> →         app: secrets</span></code>
<br>
</span><span data-lineid="408" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="409" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="410" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="411" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="412" class="logLine "><code><span> →       - name: secrets</span></code>
<br>
</span><span data-lineid="413" class="logLine "><code><span> →         image: secrets</span></code>
<br>
</span><span data-lineid="414" class="logLine "><code><span> →         command: ["/go/bin/secrets"]</span></code>
<br>
</span><span data-lineid="415" class="logLine "><code><span> →         env:</span></code>
<br>
</span><span data-lineid="416" class="logLine "><code><span> →         - name: TEMPLATE_DIR</span></code>
<br>
</span><span data-lineid="417" class="logLine "><code><span> →           value: "/go/src/github.com/tilt-dev/servantes/secrets/web/templates"</span></code>
<br>
</span><span data-lineid="418" class="logLine "><code><span> →         - name: THE_SECRET</span></code>
<br>
</span><span data-lineid="419" class="logLine "><code><span> →           valueFrom:</span></code>
<br>
</span><span data-lineid="420" class="logLine "><code><span> →             secretKeyRef:</span></code>
<br>
</span><span data-lineid="421" class="logLine "><code><span> →               name: dan-servantes-stuff</span></code>
<br>
</span><span data-lineid="422" class="logLine "><code><span> →               key: stuff</span></code>
<br>
</span><span data-lineid="423" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="424" class="logLine "><code><span> →         - containerPort: 8081</span></code>
<br>
</span><span data-lineid="425" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="426" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="427" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="428" class="logLine "><code><span> → ---</span></code>
<br>
</span><span data-lineid="429" class="logLine "><code><span> → apiVersion: v1</span></code>
<br>
</span><span data-lineid="430" class="logLine "><code><span> → kind: Service</span></code>
<br>
</span><span data-lineid="431" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="432" class="logLine "><code><span> →   name: dan-secrets</span></code>
<br>
</span><span data-lineid="433" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="434" class="logLine "><code><span> →     app: secrets</span></code>
<br>
</span><span data-lineid="435" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="436" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="437" class="logLine "><code><span> →   ports:</span></code>
<br>
</span><span data-lineid="438" class="logLine "><code><span> →   - port: 80</span></code>
<br>
</span><span data-lineid="439" class="logLine "><code><span> →     targetPort: 8081</span></code>
<br>
</span><span data-lineid="440" class="logLine "><code><span> →     protocol: TCP</span></code>
<br>
</span><span data-lineid="441" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="442" class="logLine "><code><span> →     app: secrets</span></code>
<br>
</span><span data-lineid="443" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="444" class="logLine "><code><span> → ---</span></code>
<br>
</span><span data-lineid="445" class="logLine "><code><span> → apiVersion: v1</span></code>
<br>
</span><span data-lineid="446" class="logLine "><code><span> → kind: Secret</span></code>
<br>
</span><span data-lineid="447" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="448" class="logLine "><code><span> →   name: dan-servantes-stuff</span></code>
<br>
</span><span data-lineid="449" class="logLine "><code><span> → type: Opaque</span></code>
<br>
</span><span data-lineid="450" class="logLine "><code><span> → data:</span></code>
<br>
</span><span data-lineid="451" class="logLine "><code><span> →   stuff: [redacted secret dan-servantes-stuff:stuff]</span></code>
<br>
</span><span data-lineid="452" class="logLine "><code><span> →   things: [redacted secret dan-servantes-stuff:things]</span></code>
<br>
</span><span data-lineid="453" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/job.yaml"</span></code>
<br>
</span><span data-lineid="454" class="logLine "><code><span> → apiVersion: batch/v1</span></code>
<br>
</span><span data-lineid="455" class="logLine "><code><span> → kind: Job</span></code>
<br>
</span><span data-lineid="456" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="457" class="logLine "><code><span> →   name: echo-hi</span></code>
<br>
</span><span data-lineid="458" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="459" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="460" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="461" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="462" class="logLine "><code><span> →       - name: echohi</span></code>
<br>
</span><span data-lineid="463" class="logLine "><code><span> →         image: alpine</span></code>
<br>
</span><span data-lineid="464" class="logLine "><code><span> →         command: ["echo",  "hi"]</span></code>
<br>
</span><span data-lineid="465" class="logLine "><code><span> →       restartPolicy: Never</span></code>
<br>
</span><span data-lineid="466" class="logLine "><code><span> →   backoffLimit: 4</span></code>
<br>
</span><span data-lineid="467" class="logLine "><code><span> → </span></code>
<br>
</span><span data-lineid="468" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/sleeper.yaml"</span></code>
<br>
</span><span data-lineid="469" class="logLine "><code><span> → apiVersion: v1</span></code>
<br>
</span><span data-lineid="470" class="logLine "><code><span> → kind: Pod</span></code>
<br>
</span><span data-lineid="471" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="472" class="logLine "><code><span> →  name: sleep</span></code>
<br>
</span><span data-lineid="473" class="logLine "><code><span> →  labels:</span></code>
<br>
</span><span data-lineid="474" class="logLine "><code><span> →    app: sleep</span></code>
<br>
</span><span data-lineid="475" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="476" class="logLine "><code><span> →   restartPolicy: OnFailure</span></code>
<br>
</span><span data-lineid="477" class="logLine "><code><span> →   containers:</span></code>
<br>
</span><span data-lineid="478" class="logLine "><code><span> →   - name: sleep</span></code>
<br>
</span><span data-lineid="479" class="logLine "><code><span> →     image: sleep</span></code>
<br>
</span><span data-lineid="480" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/hello_world.yaml"</span></code>
<br>
</span><span data-lineid="481" class="logLine "><code><span> → apiVersion: apps/v1</span></code>
<br>
</span><span data-lineid="482" class="logLine "><code><span> → kind: Deployment</span></code>
<br>
</span><span data-lineid="483" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="484" class="logLine "><code><span> →   name: hello-world</span></code>
<br>
</span><span data-lineid="485" class="logLine "><code><span> →   labels:</span></code>
<br>
</span><span data-lineid="486" class="logLine "><code><span> →     app: hello-world</span></code>
<br>
</span><span data-lineid="487" class="logLine "><code><span> →     owner: dan</span></code>
<br>
</span><span data-lineid="488" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="489" class="logLine "><code><span> →   selector:</span></code>
<br>
</span><span data-lineid="490" class="logLine "><code><span> →     matchLabels:</span></code>
<br>
</span><span data-lineid="491" class="logLine "><code><span> →       app: hello-world</span></code>
<br>
</span><span data-lineid="492" class="logLine "><code><span> →       owner: dan</span></code>
<br>
</span><span data-lineid="493" class="logLine "><code><span> →   template:</span></code>
<br>
</span><span data-lineid="494" class="logLine "><code><span> →     metadata:</span></code>
<br>
</span><span data-lineid="495" class="logLine "><code><span> →       labels:</span></code>
<br>
</span><span data-lineid="496" class="logLine "><code><span> →         app: hello-world</span></code>
<br>
</span><span data-lineid="497" class="logLine "><code><span> →         tier: web</span></code>
<br>
</span><span data-lineid="498" class="logLine "><code><span> →         owner: dan</span></code>
<br>
</span><span data-lineid="499" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="500" class="logLine "><code><span> →       containers:</span></code>
<br>
</span><span data-lineid="501" class="logLine "><code><span> →       - name: hello-world</span></code>
<br>
</span><span data-lineid="502" class="logLine "><code><span> →         image: strm/helloworld-http</span></code>
<br>
</span><span data-lineid="503" class="logLine "><code><span> →         ports:</span></code>
<br>
</span><span data-lineid="504" class="logLine "><code><span> →         - containerPort: 80</span></code>
<br>
</span><span data-lineid="505" class="logLine "><code><span> →         resources:</span></code>
<br>
</span><span data-lineid="506" class="logLine "><code><span> →           requests:</span></code>
<br>
</span><span data-lineid="507" class="logLine "><code><span> →             cpu: "10m"</span></code>
<br>
</span><span data-lineid="508" class="logLine "><code><span>local: m4 -Dvarowner=dan "deploy/tick.yaml"</span></code>
<br>
</span><span data-lineid="509" class="logLine "><code><span> → apiVersion: batch/v1beta1</span></code>
<br>
</span><span data-lineid="510" class="logLine "><code><span> → kind: CronJob</span></code>
<br>
</span><span data-lineid="511" class="logLine "><code><span> → metadata:</span></code>
<br>
</span><span data-lineid="512" class="logLine "><code><span> →   name: tick</span></code>
<br>
</span><span data-lineid="513" class="logLine "><code><span> → spec:</span></code>
<br>
</span><span data-lineid="514" class="logLine "><code><span> →   schedule: "*/1 * * * *"</span></code>
<br>
</span><span data-lineid="515" class="logLine "><code><span> →   startingDeadlineSeconds: 90</span></code>
<br>
</span><span data-lineid="516" class="logLine "><code><span> →   jobTemplate:</span></code>
<br>
</span><span data-lineid="517" class="logLine "><code><span> →     spec:</span></code>
<br>
</span><span data-lineid="518" class="logLine "><code><span> →       template:</span></code>
<br>
</span><span data-lineid="519" class="logLine "><code><span> →         spec:</span></code>
<br>
</span><span data-lineid="520" class="logLine "><code><span> →           containers:</span></code>
<br>
</span><span data-lineid="521" class="logLine "><code><span> →           - name: tick</span></code>
<br>
</span><span data-lineid="522" class="logLine "><code><span> →             image: busybox</span></code>
<br>
</span><span data-lineid="523" class="logLine "><code><span> →             args:</span></code>
<br>
</span><span data-lineid="524" class="logLine "><code><span> →             - /bin/sh</span></code>
<br>
</span><span data-lineid="525" class="logLine "><code><span> →             - -c</span></code>
<br>
</span><span data-lineid="526" class="logLine "><code><span> →             - date; echo tick</span></code>
<br>
</span><span data-lineid="527" class="logLine "><code><span> →           restartPolicy: OnFailure</span></code>
<br>
</span><span data-lineid="528" class="logLine "><code><span>Successfully loaded Tiltfile (278.651752ms)</span></code>
<br>
</span><span data-lineid="529" class="logLine "><code><span>uncategoriz…┊ </span></code>
<br>
</span><span data-lineid="530" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">──┤ Building: </span><span>uncategorized</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="531" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">STEP 1/1 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="532" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="533" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="534" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">  │ </span><span>   pod-reader:role</span></code>
<br>
</span><span data-lineid="535" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">  │ </span><span>   read-pods:rolebinding</span></code>
<br>
</span><span data-lineid="536" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">  │ </span><span>   dan-servantes-stuff:secret</span></code>
<br>
</span><span data-lineid="537" class="logLine "><code><span>uncategoriz…┊ </span></code>
<br>
</span><span data-lineid="538" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.321s</span></code>
<br>
</span><span data-lineid="539" class="logLine "><code><span>uncategoriz…┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.321s </span></code>
<br>
</span><span data-lineid="540" class="logLine "><code><span>uncategoriz…┊ </span></code>
<br>
</span><span data-lineid="541" class="logLine "><code><span>echo-hi     ┊ </span></code>
<br>
</span><span data-lineid="542" class="logLine "><code><span>echo-hi     ┊ </span><span class="ansi-blue">──┤ Building: </span><span>echo-hi</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="543" class="logLine "><code><span>echo-hi     ┊ </span><span class="ansi-blue">STEP 1/1 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="544" class="logLine "><code><span>echo-hi     ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="545" class="logLine "><code><span>echo-hi     ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="546" class="logLine "><code><span>echo-hi     ┊ </span><span class="ansi-blue">  │ </span><span>   echo-hi:job</span></code>
<br>
</span><span data-lineid="547" class="logLine "><code><span>echo-hi     ┊ </span></code>
<br>
</span><span data-lineid="548" class="logLine "><code><span>echo-hi     ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.221s</span></code>
<br>
</span><span data-lineid="549" class="logLine "><code><span>echo-hi     ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.221s </span></code>
<br>
</span><span data-lineid="550" class="logLine "><code><span>echo-hi     ┊ </span></code>
<br>
</span><span data-lineid="551" class="logLine "><code><span>hello-world ┊ </span></code>
<br>
</span><span data-lineid="552" class="logLine "><code><span>hello-world ┊ </span><span class="ansi-blue">──┤ Building: </span><span>hello-world</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="553" class="logLine "><code><span>hello-world ┊ </span><span class="ansi-blue">STEP 1/1 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="554" class="logLine "><code><span>hello-world ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="555" class="logLine "><code><span>hello-world ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="556" class="logLine "><code><span>hello-world ┊ </span><span class="ansi-blue">  │ </span><span>   hello-world:deployment</span></code>
<br>
</span><span data-lineid="557" class="logLine "><code><span>hello-world ┊ </span></code>
<br>
</span><span data-lineid="558" class="logLine "><code><span>hello-world ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.197s</span></code>
<br>
</span><span data-lineid="559" class="logLine "><code><span>hello-world ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.197s </span></code>
<br>
</span><span data-lineid="560" class="logLine "><code><span>hello-world ┊ </span></code>
<br>
</span><span data-lineid="561" class="logLine "><code><span>tick        ┊ </span></code>
<br>
</span><span data-lineid="562" class="logLine "><code><span>tick        ┊ </span><span class="ansi-blue">──┤ Building: </span><span>tick</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="563" class="logLine "><code><span>tick        ┊ </span><span class="ansi-blue">STEP 1/1 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="564" class="logLine "><code><span>tick        ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="565" class="logLine "><code><span>tick        ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="566" class="logLine "><code><span>tick        ┊ </span><span class="ansi-blue">  │ </span><span>   tick:cronjob</span></code>
<br>
</span><span data-lineid="567" class="logLine "><code><span>Starting Tilt webpack server…</span></code>
<br>
</span><span data-lineid="568" class="logLine "><code><span>tick        ┊ </span></code>
<br>
</span><span data-lineid="569" class="logLine "><code><span>tick        ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.246s</span></code>
<br>
</span><span data-lineid="570" class="logLine "><code><span>tick        ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.246s </span></code>
<br>
</span><span data-lineid="571" class="logLine "><code><span>tick        ┊ </span></code>
<br>
</span><span data-lineid="572" class="logLine "><code><span>fe          ┊ </span></code>
<br>
</span><span data-lineid="573" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">──┤ Building: </span><span>fe</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="574" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [fe]</span></code>
<br>
</span><span data-lineid="575" class="logLine "><code><span>fe          ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="576" class="logLine "><code><span>fe          ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="577" class="logLine "><code><span>fe          ┊   </span></code>
<br>
</span><span data-lineid="578" class="logLine "><code><span>fe          ┊   RUN apt update &amp;&amp; apt install -y unzip time make</span></code>
<br>
</span><span data-lineid="579" class="logLine "><code><span>fe          ┊   </span></code>
<br>
</span><span data-lineid="580" class="logLine "><code><span>fe          ┊   ENV PROTOC_VERSION 3.5.1</span></code>
<br>
</span><span data-lineid="581" class="logLine "><code><span>fe          ┊   </span></code>
<br>
</span><span data-lineid="582" class="logLine "><code><span>fe          ┊   RUN wget https://github.com/google/protobuf/releases/download/v$</span></code>
<br>
</span><span data-lineid="583" class="logLine "><code><span>fe          ┊     unzip protoc--linux-x86_64.zip -d protoc &amp;&amp; </span></code>
<br>
</span><span data-lineid="584" class="logLine "><code><span>fe          ┊     mv protoc/bin/protoc /usr/bin/protoc</span></code>
<br>
</span><span data-lineid="585" class="logLine "><code><span>fe          ┊   </span></code>
<br>
</span><span data-lineid="586" class="logLine "><code><span>fe          ┊   RUN go get github.com/golang/protobuf/protoc-gen-go</span></code>
<br>
</span><span data-lineid="587" class="logLine "><code><span>fe          ┊   </span></code>
<br>
</span><span data-lineid="588" class="logLine "><code><span>fe          ┊   ADD . /go/src/github.com/tilt-dev/servantes/fe</span></code>
<br>
</span><span data-lineid="589" class="logLine "><code><span>fe          ┊   RUN go install github.com/tilt-dev/servantes/fe</span></code>
<br>
</span><span data-lineid="590" class="logLine "><code><span>fe          ┊   ENTRYPOINT /go/bin/fe</span></code>
<br>
</span><span data-lineid="591" class="logLine "><code><span>fe          ┊ </span></code>
<br>
</span><span data-lineid="592" class="logLine "><code><span>fe          ┊ </span></code>
<br>
</span><span data-lineid="593" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="594" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="595" class="logLine "><code><span>fe          ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="596" class="logLine "><code><span>fe          ┊     ╎ copy /context / done | 503ms</span></code>
<br>
</span><span data-lineid="597" class="logLine "><code><span>fe          ┊     ╎ [1/6] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="598" class="logLine "><code><span>fe          ┊     ╎ [cached] [2/6] RUN apt update &amp;&amp; apt install -y unzip time make</span></code>
<br>
</span><span data-lineid="599" class="logLine "><code><span>fe          ┊     ╎ [cached] [3/6] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &amp;&amp;   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &amp;&amp;   mv protoc/bin/protoc /usr/bin/protoc</span></code>
<br>
</span><span data-lineid="600" class="logLine "><code><span>fe          ┊     ╎ [cached] [4/6] RUN go get github.com/golang/protobuf/protoc-gen-go</span></code>
<br>
</span><span data-lineid="601" class="logLine "><code><span>fe          ┊     ╎ [cached] [5/6] ADD . /go/src/github.com/tilt-dev/servantes/fe</span></code>
<br>
</span><span data-lineid="602" class="logLine "><code><span>fe          ┊     ╎ [cached] [6/6] RUN go install github.com/tilt-dev/servantes/fe</span></code>
<br>
</span><span data-lineid="603" class="logLine "><code><span>fe          ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="604" class="logLine "><code><span>fe          ┊ </span></code>
<br>
</span><span data-lineid="605" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/fe:tilt-2aa0d1ffc977ad93</span></code>
<br>
</span><span data-lineid="606" class="logLine "><code><span>fe          ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="607" class="logLine "><code><span>fe          ┊ </span></code>
<br>
</span><span data-lineid="608" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="609" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="610" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="611" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>   dan-fe:service</span></code>
<br>
</span><span data-lineid="612" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>   dan-fe:deployment</span></code>
<br>
</span><span data-lineid="613" class="logLine "><code><span>fe          ┊ </span></code>
<br>
</span><span data-lineid="614" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 2.656s</span></code>
<br>
</span><span data-lineid="615" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="616" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.237s</span></code>
<br>
</span><span data-lineid="617" class="logLine "><code><span>fe          ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 2.893s </span></code>
<br>
</span><span data-lineid="618" class="logLine "><code><span>fe          ┊ </span></code>
<br>
</span><span data-lineid="619" class="logLine "><code><span>vigoda      ┊ </span></code>
<br>
</span><span data-lineid="620" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">──┤ Building: </span><span>vigoda</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="621" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [vigoda]</span></code>
<br>
</span><span data-lineid="622" class="logLine "><code><span>vigoda      ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="623" class="logLine "><code><span>vigoda      ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="624" class="logLine "><code><span>vigoda      ┊   </span></code>
<br>
</span><span data-lineid="625" class="logLine "><code><span>vigoda      ┊   ADD . /go/src/github.com/tilt-dev/servantes/vigoda</span></code>
<br>
</span><span data-lineid="626" class="logLine "><code><span>vigoda      ┊   RUN go install github.com/tilt-dev/servantes/vigoda</span></code>
<br>
</span><span data-lineid="627" class="logLine "><code><span>vigoda      ┊   </span></code>
<br>
</span><span data-lineid="628" class="logLine "><code><span>vigoda      ┊   ENTRYPOINT /go/bin/vigoda</span></code>
<br>
</span><span data-lineid="629" class="logLine "><code><span>vigoda      ┊ </span></code>
<br>
</span><span data-lineid="630" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="631" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="632" class="logLine "><code><span>vigoda      ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="633" class="logLine "><code><span>vigoda      ┊     ╎ copy /context / done | 265ms</span></code>
<br>
</span><span data-lineid="634" class="logLine "><code><span>vigoda      ┊     ╎ [1/3] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="635" class="logLine "><code><span>vigoda      ┊     ╎ [cached] [2/3] ADD . /go/src/github.com/tilt-dev/servantes/vigoda</span></code>
<br>
</span><span data-lineid="636" class="logLine "><code><span>vigoda      ┊     ╎ [cached] [3/3] RUN go install github.com/tilt-dev/servantes/vigoda</span></code>
<br>
</span><span data-lineid="637" class="logLine "><code><span>vigoda      ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="638" class="logLine "><code><span>vigoda      ┊ </span></code>
<br>
</span><span data-lineid="639" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/vigoda:tilt-4b054fd2cc2e35cc</span></code>
<br>
</span><span data-lineid="640" class="logLine "><code><span>vigoda      ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="641" class="logLine "><code><span>vigoda      ┊ </span></code>
<br>
</span><span data-lineid="642" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="643" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="644" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="645" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>   dan-vigoda:deployment</span></code>
<br>
</span><span data-lineid="646" class="logLine "><code><span>vigoda      ┊ </span></code>
<br>
</span><span data-lineid="647" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.566s</span></code>
<br>
</span><span data-lineid="648" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="649" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.298s</span></code>
<br>
</span><span data-lineid="650" class="logLine "><code><span>vigoda      ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.864s </span></code>
<br>
</span><span data-lineid="651" class="logLine "><code><span>vigoda      ┊ </span></code>
<br>
</span><span data-lineid="652" class="logLine "><code><span>snack       ┊ </span></code>
<br>
</span><span data-lineid="653" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">──┤ Building: </span><span>snack</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="654" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [snack]</span></code>
<br>
</span><span data-lineid="655" class="logLine "><code><span>snack       ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="656" class="logLine "><code><span>snack       ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="657" class="logLine "><code><span>snack       ┊   </span></code>
<br>
</span><span data-lineid="658" class="logLine "><code><span>snack       ┊   # TODO(dbentley): this is only relevant in devel; factor this out to a separate image</span></code>
<br>
</span><span data-lineid="659" class="logLine "><code><span>snack       ┊   RUN git clone https://github.com/tilt-dev/rerun-process-wrapper</span></code>
<br>
</span><span data-lineid="660" class="logLine "><code><span>snack       ┊   RUN cp rerun-process-wrapper/*.sh /bin</span></code>
<br>
</span><span data-lineid="661" class="logLine "><code><span>snack       ┊   </span></code>
<br>
</span><span data-lineid="662" class="logLine "><code><span>snack       ┊   ADD . /go/src/github.com/tilt-dev/servantes/snack</span></code>
<br>
</span><span data-lineid="663" class="logLine "><code><span>snack       ┊   RUN go install github.com/tilt-dev/servantes/snack</span></code>
<br>
</span><span data-lineid="664" class="logLine "><code><span>snack       ┊   </span></code>
<br>
</span><span data-lineid="665" class="logLine "><code><span>snack       ┊   ENTRYPOINT /bin/start.sh /go/bin/snack</span></code>
<br>
</span><span data-lineid="666" class="logLine "><code><span>snack       ┊ </span></code>
<br>
</span><span data-lineid="667" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="668" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="669" class="logLine "><code><span>snack       ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="670" class="logLine "><code><span>snack       ┊     ╎ copy /context / done | 268ms</span></code>
<br>
</span><span data-lineid="671" class="logLine "><code><span>snack       ┊     ╎ [1/5] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="672" class="logLine "><code><span>snack       ┊     ╎ [cached] [2/5] RUN git clone https://github.com/tilt-dev/rerun-process-wrapper</span></code>
<br>
</span><span data-lineid="673" class="logLine "><code><span>snack       ┊     ╎ [cached] [3/5] RUN cp rerun-process-wrapper/*.sh /bin</span></code>
<br>
</span><span data-lineid="674" class="logLine "><code><span>snack       ┊     ╎ [cached] [4/5] ADD . /go/src/github.com/tilt-dev/servantes/snack</span></code>
<br>
</span><span data-lineid="675" class="logLine "><code><span>snack       ┊     ╎ [cached] [5/5] RUN go install github.com/tilt-dev/servantes/snack</span></code>
<br>
</span><span data-lineid="676" class="logLine "><code><span>snack       ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="677" class="logLine "><code><span>snack       ┊ </span></code>
<br>
</span><span data-lineid="678" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/snack:tilt-74df851b93d949de</span></code>
<br>
</span><span data-lineid="679" class="logLine "><code><span>snack       ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="680" class="logLine "><code><span>snack       ┊ </span></code>
<br>
</span><span data-lineid="681" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="682" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="683" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="684" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>   dan-snack:deployment</span></code>
<br>
</span><span data-lineid="685" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:33:47 Server status: All good</span></code>
<br>
</span><span data-lineid="686" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:33:49 Server status: All good</span></code>
<br>
</span><span data-lineid="687" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:33:51 Server status: All good</span></code>
<br>
</span><span data-lineid="688" class="logLine "><code><span>snack       ┊ </span></code>
<br>
</span><span data-lineid="689" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.893s</span></code>
<br>
</span><span data-lineid="690" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="691" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.298s</span></code>
<br>
</span><span data-lineid="692" class="logLine "><code><span>snack       ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 1.191s </span></code>
<br>
</span><span data-lineid="693" class="logLine "><code><span>snack       ┊ </span></code>
<br>
</span><span data-lineid="694" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="695" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">──┤ Building: </span><span>doggos</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="696" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">STEP 1/5 — </span><span>Building Dockerfile: [doggos]</span></code>
<br>
</span><span data-lineid="697" class="logLine "><code><span>doggos      ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="698" class="logLine "><code><span>doggos      ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="699" class="logLine "><code><span>doggos      ┊   </span></code>
<br>
</span><span data-lineid="700" class="logLine "><code><span>doggos      ┊   ADD . /go/src/github.com/tilt-dev/servantes/doggos</span></code>
<br>
</span><span data-lineid="701" class="logLine "><code><span>doggos      ┊   RUN go install github.com/tilt-dev/servantes/doggos</span></code>
<br>
</span><span data-lineid="702" class="logLine "><code><span>doggos      ┊   </span></code>
<br>
</span><span data-lineid="703" class="logLine "><code><span>doggos      ┊   ENTRYPOINT /go/bin/doggos</span></code>
<br>
</span><span data-lineid="704" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="705" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="706" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="707" class="logLine "><code><span>doggos      ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="708" class="logLine "><code><span>doggos      ┊     ╎ copy /context / done | 382ms</span></code>
<br>
</span><span data-lineid="709" class="logLine "><code><span>doggos      ┊     ╎ [1/3] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="710" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:33:53 Server status: All good</span></code>
<br>
</span><span data-lineid="711" class="logLine "><code><span>doggos      ┊     ╎ [cached] [2/3] ADD . /go/src/github.com/tilt-dev/servantes/doggos</span></code>
<br>
</span><span data-lineid="712" class="logLine "><code><span>doggos      ┊     ╎ [cached] [3/3] RUN go install github.com/tilt-dev/servantes/doggos</span></code>
<br>
</span><span data-lineid="713" class="logLine "><code><span>doggos      ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="714" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="715" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">STEP 2/5 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/doggos:tilt-3e2b479f1b36d94d</span></code>
<br>
</span><span data-lineid="716" class="logLine "><code><span>doggos      ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="717" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="718" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">STEP 3/5 — </span><span>Building Dockerfile: [sidecar]</span></code>
<br>
</span><span data-lineid="719" class="logLine "><code><span>doggos      ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="720" class="logLine "><code><span>doggos      ┊   FROM rust:1.37.0-alpine</span></code>
<br>
</span><span data-lineid="721" class="logLine "><code><span>doggos      ┊   </span></code>
<br>
</span><span data-lineid="722" class="logLine "><code><span>doggos      ┊   COPY ./ ./</span></code>
<br>
</span><span data-lineid="723" class="logLine "><code><span>doggos      ┊   </span></code>
<br>
</span><span data-lineid="724" class="logLine "><code><span>doggos      ┊   RUN cargo build --release</span></code>
<br>
</span><span data-lineid="725" class="logLine "><code><span>doggos      ┊   CMD target/release/sidecar</span></code>
<br>
</span><span data-lineid="726" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="727" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="728" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="729" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="730" class="logLine "><code><span>doggos      ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="731" class="logLine "><code><span>doggos      ┊     ╎ copy /context / done | 333ms</span></code>
<br>
</span><span data-lineid="732" class="logLine "><code><span>doggos      ┊     ╎ [1/3] FROM docker.io/library/rust:1.37.0-alpine@sha256:902923b8aff23b9b73011517805df98194ead477dab040b4aea4104aa9041058</span></code>
<br>
</span><span data-lineid="733" class="logLine "><code><span>doggos      ┊     ╎ [cached] [2/3] COPY ./ ./</span></code>
<br>
</span><span data-lineid="734" class="logLine "><code><span>doggos      ┊     ╎ [cached] [3/3] RUN cargo build --release</span></code>
<br>
</span><span data-lineid="735" class="logLine "><code><span>doggos      ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="736" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="737" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">STEP 4/5 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/sidecar:tilt-55f6a6f9333378b3</span></code>
<br>
</span><span data-lineid="738" class="logLine "><code><span>doggos      ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="739" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="740" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">STEP 5/5 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="741" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="742" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="743" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>   dan-doggos:deployment</span></code>
<br>
</span><span data-lineid="744" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="745" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 1.022s</span></code>
<br>
</span><span data-lineid="746" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="747" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.983s</span></code>
<br>
</span><span data-lineid="748" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Step 4 - 0.000s</span></code>
<br>
</span><span data-lineid="749" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Step 5 - 0.205s</span></code>
<br>
</span><span data-lineid="750" class="logLine "><code><span>doggos      ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 2.211s </span></code>
<br>
</span><span data-lineid="751" class="logLine "><code><span>doggos      ┊ </span></code>
<br>
</span><span data-lineid="752" class="logLine "><code><span>fortune     ┊ </span></code>
<br>
</span><span data-lineid="753" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">──┤ Building: </span><span>fortune</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="754" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [fortune]</span></code>
<br>
</span><span data-lineid="755" class="logLine "><code><span>fortune     ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="756" class="logLine "><code><span>fortune     ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="757" class="logLine "><code><span>fortune     ┊   </span></code>
<br>
</span><span data-lineid="758" class="logLine "><code><span>fortune     ┊   RUN apt update &amp;&amp; apt install -y unzip time make</span></code>
<br>
</span><span data-lineid="759" class="logLine "><code><span>fortune     ┊   </span></code>
<br>
</span><span data-lineid="760" class="logLine "><code><span>fortune     ┊   ENV PROTOC_VERSION 3.5.1</span></code>
<br>
</span><span data-lineid="761" class="logLine "><code><span>fortune     ┊   </span></code>
<br>
</span><span data-lineid="762" class="logLine "><code><span>fortune     ┊   RUN wget https://github.com/google/protobuf/releases/download/v$ </span></code>
<br>
</span><span data-lineid="763" class="logLine "><code><span>fortune     ┊     unzip protoc-linux-x86_64.zip -d protoc &amp;&amp; </span></code>
<br>
</span><span data-lineid="764" class="logLine "><code><span>fortune     ┊     mv protoc/bin/protoc /usr/bin/protoc</span></code>
<br>
</span><span data-lineid="765" class="logLine "><code><span>fortune     ┊   </span></code>
<br>
</span><span data-lineid="766" class="logLine "><code><span>fortune     ┊   RUN go get github.com/golang/protobuf/protoc-gen-go</span></code>
<br>
</span><span data-lineid="767" class="logLine "><code><span>fortune     ┊   </span></code>
<br>
</span><span data-lineid="768" class="logLine "><code><span>fortune     ┊   ADD . /go/src/github.com/tilt-dev/servantes/fortune</span></code>
<br>
</span><span data-lineid="769" class="logLine "><code><span>fortune     ┊   RUN cd /go/src/github.com/tilt-dev/servantes/fortune &amp;&amp; make proto</span></code>
<br>
</span><span data-lineid="770" class="logLine "><code><span>fortune     ┊   RUN go install github.com/tilt-dev/servantes/fortune</span></code>
<br>
</span><span data-lineid="771" class="logLine "><code><span>fortune     ┊   </span></code>
<br>
</span><span data-lineid="772" class="logLine "><code><span>fortune     ┊   ENTRYPOINT /go/bin/fortune</span></code>
<br>
</span><span data-lineid="773" class="logLine "><code><span>fortune     ┊ </span></code>
<br>
</span><span data-lineid="774" class="logLine "><code><span>fortune     ┊ </span></code>
<br>
</span><span data-lineid="775" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="776" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="777" class="logLine "><code><span>fortune     ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="778" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="779" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="780" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="781" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="782" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="783" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:33:48 Heartbeat</span></code>
<br>
</span><span data-lineid="784" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:33:52 Heartbeat</span></code>
<br>
</span><span data-lineid="785" class="logLine "><code><span>fortune     ┊     ╎ copy /context / done | 275ms</span></code>
<br>
</span><span data-lineid="786" class="logLine "><code><span>fortune     ┊     ╎ [1/7] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="787" class="logLine "><code><span>fortune     ┊     ╎ [cached] [2/7] RUN apt update &amp;&amp; apt install -y unzip time make</span></code>
<br>
</span><span data-lineid="788" class="logLine "><code><span>fortune     ┊     ╎ [cached] [3/7] RUN wget https://github.com/google/protobuf/releases/download/v3.5.1/protoc-3.5.1-linux-x86_64.zip &amp;&amp;   unzip protoc-3.5.1-linux-x86_64.zip -d protoc &amp;&amp;   mv protoc/bin/protoc /usr/bin/protoc</span></code>
<br>
</span><span data-lineid="789" class="logLine "><code><span>fortune     ┊     ╎ [cached] [4/7] RUN go get github.com/golang/protobuf/protoc-gen-go</span></code>
<br>
</span><span data-lineid="790" class="logLine "><code><span>fortune     ┊     ╎ [cached] [5/7] ADD . /go/src/github.com/tilt-dev/servantes/fortune</span></code>
<br>
</span><span data-lineid="791" class="logLine "><code><span>fortune     ┊     ╎ [cached] [6/7] RUN cd /go/src/github.com/tilt-dev/servantes/fortune &amp;&amp; make proto</span></code>
<br>
</span><span data-lineid="792" class="logLine "><code><span>fortune     ┊     ╎ [cached] [7/7] RUN go install github.com/tilt-dev/servantes/fortune</span></code>
<br>
</span><span data-lineid="793" class="logLine "><code><span>fortune     ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="794" class="logLine "><code><span>fortune     ┊ </span></code>
<br>
</span><span data-lineid="795" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/fortune:tilt-6709c385b11f291e</span></code>
<br>
</span><span data-lineid="796" class="logLine "><code><span>fortune     ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="797" class="logLine "><code><span>fortune     ┊ </span></code>
<br>
</span><span data-lineid="798" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="799" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="800" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="801" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>   dan-fortune:deployment</span></code>
<br>
</span><span data-lineid="802" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:33:55 Server status: All good</span></code>
<br>
</span><span data-lineid="803" class="logLine "><code><span>fortune     ┊ </span></code>
<br>
</span><span data-lineid="804" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.589s</span></code>
<br>
</span><span data-lineid="805" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="806" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.194s</span></code>
<br>
</span><span data-lineid="807" class="logLine "><code><span>fortune     ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.783s </span></code>
<br>
</span><span data-lineid="808" class="logLine "><code><span>fortune     ┊ </span></code>
<br>
</span><span data-lineid="809" class="logLine "><code><span>hypothesizer┊ </span></code>
<br>
</span><span data-lineid="810" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">──┤ Building: </span><span>hypothesizer</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="811" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [hypothesizer]</span></code>
<br>
</span><span data-lineid="812" class="logLine "><code><span>hypothesizer┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="813" class="logLine "><code><span>hypothesizer┊   FROM python:3.6</span></code>
<br>
</span><span data-lineid="814" class="logLine "><code><span>hypothesizer┊   </span></code>
<br>
</span><span data-lineid="815" class="logLine "><code><span>hypothesizer┊   ADD . /app</span></code>
<br>
</span><span data-lineid="816" class="logLine "><code><span>hypothesizer┊   RUN cd /app &amp;&amp; pip install -r requirements.txt</span></code>
<br>
</span><span data-lineid="817" class="logLine "><code><span>hypothesizer┊ </span></code>
<br>
</span><span data-lineid="818" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="819" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="820" class="logLine "><code><span>hypothesizer┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="821" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:33:55 Heartbeat</span></code>
<br>
</span><span data-lineid="822" class="logLine "><code><span>hypothesizer┊     ╎ copy /context / done | 260ms</span></code>
<br>
</span><span data-lineid="823" class="logLine "><code><span>hypothesizer┊     ╎ [1/3] FROM docker.io/library/python:3.6@sha256:52f872eae9755743c9494e0e3cf02a47d34b42032cab1e5ab777b30c3665d5f1</span></code>
<br>
</span><span data-lineid="824" class="logLine "><code><span>hypothesizer┊     ╎ [cached] [2/3] ADD . /app</span></code>
<br>
</span><span data-lineid="825" class="logLine "><code><span>hypothesizer┊     ╎ [cached] [3/3] RUN cd /app &amp;&amp; pip install -r requirements.txt</span></code>
<br>
</span><span data-lineid="826" class="logLine "><code><span>hypothesizer┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="827" class="logLine "><code><span>hypothesizer┊ </span></code>
<br>
</span><span data-lineid="828" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/hypothesizer:tilt-7d94bb074b743c3b</span></code>
<br>
</span><span data-lineid="829" class="logLine "><code><span>hypothesizer┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="830" class="logLine "><code><span>hypothesizer┊ </span></code>
<br>
</span><span data-lineid="831" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="832" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="833" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="834" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>   dan-hypothesizer:service</span></code>
<br>
</span><span data-lineid="835" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>   dan-hypothesizer:deployment</span></code>
<br>
</span><span data-lineid="836" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="837" class="logLine "><code><span>hypothesizer┊ </span></code>
<br>
</span><span data-lineid="838" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.709s</span></code>
<br>
</span><span data-lineid="839" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="840" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.238s</span></code>
<br>
</span><span data-lineid="841" class="logLine "><code><span>hypothesizer┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.947s </span></code>
<br>
</span><span data-lineid="842" class="logLine "><code><span>hypothesizer┊ </span></code>
<br>
</span><span data-lineid="843" class="logLine "><code><span>spoonerisms ┊ </span></code>
<br>
</span><span data-lineid="844" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">──┤ Building: </span><span>spoonerisms</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="845" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [spoonerisms]</span></code>
<br>
</span><span data-lineid="846" class="logLine "><code><span>spoonerisms ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="847" class="logLine "><code><span>spoonerisms ┊   FROM node:10</span></code>
<br>
</span><span data-lineid="848" class="logLine "><code><span>spoonerisms ┊   </span></code>
<br>
</span><span data-lineid="849" class="logLine "><code><span>spoonerisms ┊   ADD package.json /app/package.json</span></code>
<br>
</span><span data-lineid="850" class="logLine "><code><span>spoonerisms ┊   ADD yarn.lock /app/yarn.lock</span></code>
<br>
</span><span data-lineid="851" class="logLine "><code><span>spoonerisms ┊   RUN cd /app &amp;&amp; yarn install</span></code>
<br>
</span><span data-lineid="852" class="logLine "><code><span>spoonerisms ┊   </span></code>
<br>
</span><span data-lineid="853" class="logLine "><code><span>spoonerisms ┊   ADD src /app</span></code>
<br>
</span><span data-lineid="854" class="logLine "><code><span>spoonerisms ┊   </span></code>
<br>
</span><span data-lineid="855" class="logLine "><code><span>spoonerisms ┊   ENTRYPOINT [ "node", "/app/index.js" ]</span></code>
<br>
</span><span data-lineid="856" class="logLine "><code><span>spoonerisms ┊ </span></code>
<br>
</span><span data-lineid="857" class="logLine "><code><span>spoonerisms ┊ </span></code>
<br>
</span><span data-lineid="858" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="859" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="860" class="logLine "><code><span>spoonerisms ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="861" class="logLine "><code><span>spoonerisms ┊     ╎ copy /context / done | 225ms</span></code>
<br>
</span><span data-lineid="862" class="logLine "><code><span>spoonerisms ┊     ╎ [1/5] FROM docker.io/library/node:10@sha256:dabc15ad36a9e0a95862fbdf6ffdad439edc20aa27c7f10456644464e3fb5f08</span></code>
<br>
</span><span data-lineid="863" class="logLine "><code><span>spoonerisms ┊     ╎ [cached] [2/5] ADD package.json /app/package.json</span></code>
<br>
</span><span data-lineid="864" class="logLine "><code><span>spoonerisms ┊     ╎ [cached] [3/5] ADD yarn.lock /app/yarn.lock</span></code>
<br>
</span><span data-lineid="865" class="logLine "><code><span>spoonerisms ┊     ╎ [cached] [4/5] RUN cd /app &amp;&amp; yarn install</span></code>
<br>
</span><span data-lineid="866" class="logLine "><code><span>spoonerisms ┊     ╎ [cached] [5/5] ADD src /app</span></code>
<br>
</span><span data-lineid="867" class="logLine "><code><span>spoonerisms ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="868" class="logLine "><code><span>spoonerisms ┊ </span></code>
<br>
</span><span data-lineid="869" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/spoonerisms:tilt-a54503fe0fc32427</span></code>
<br>
</span><span data-lineid="870" class="logLine "><code><span>spoonerisms ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="871" class="logLine "><code><span>spoonerisms ┊ </span></code>
<br>
</span><span data-lineid="872" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="873" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="874" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="875" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>   dan-spoonerisms:deployment</span></code>
<br>
</span><span data-lineid="876" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:33:57 Server status: All good</span></code>
<br>
</span><span data-lineid="877" class="logLine "><code><span>spoonerisms ┊ </span></code>
<br>
</span><span data-lineid="878" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.709s</span></code>
<br>
</span><span data-lineid="879" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="880" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.256s</span></code>
<br>
</span><span data-lineid="881" class="logLine "><code><span>spoonerisms ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.965s </span></code>
<br>
</span><span data-lineid="882" class="logLine "><code><span>spoonerisms ┊ </span></code>
<br>
</span><span data-lineid="883" class="logLine "><code><span>emoji       ┊ </span></code>
<br>
</span><span data-lineid="884" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">──┤ Building: </span><span>emoji</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="885" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [emoji]</span></code>
<br>
</span><span data-lineid="886" class="logLine "><code><span>emoji       ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="887" class="logLine "><code><span>emoji       ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="888" class="logLine "><code><span>emoji       ┊   </span></code>
<br>
</span><span data-lineid="889" class="logLine "><code><span>emoji       ┊   ADD . /go/src/github.com/tilt-dev/servantes/emoji</span></code>
<br>
</span><span data-lineid="890" class="logLine "><code><span>emoji       ┊   RUN go install github.com/tilt-dev/servantes/emoji</span></code>
<br>
</span><span data-lineid="891" class="logLine "><code><span>emoji       ┊   </span></code>
<br>
</span><span data-lineid="892" class="logLine "><code><span>emoji       ┊   ENTRYPOINT /go/bin/emoji</span></code>
<br>
</span><span data-lineid="893" class="logLine "><code><span>emoji       ┊ </span></code>
<br>
</span><span data-lineid="894" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="895" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="896" class="logLine "><code><span>emoji       ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="897" class="logLine "><code><span>emoji       ┊     ╎ copy /context / done | 361ms</span></code>
<br>
</span><span data-lineid="898" class="logLine "><code><span>emoji       ┊     ╎ [1/3] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="899" class="logLine "><code><span>emoji       ┊     ╎ [cached] [2/3] ADD . /go/src/github.com/tilt-dev/servantes/emoji</span></code>
<br>
</span><span data-lineid="900" class="logLine "><code><span>emoji       ┊     ╎ [cached] [3/3] RUN go install github.com/tilt-dev/servantes/emoji</span></code>
<br>
</span><span data-lineid="901" class="logLine "><code><span>emoji       ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="902" class="logLine "><code><span>emoji       ┊ </span></code>
<br>
</span><span data-lineid="903" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/emoji:tilt-9d569286c39378d4</span></code>
<br>
</span><span data-lineid="904" class="logLine "><code><span>emoji       ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="905" class="logLine "><code><span>emoji       ┊ </span></code>
<br>
</span><span data-lineid="906" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="907" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="908" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="909" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>   dan-emoji:deployment</span></code>
<br>
</span><span data-lineid="910" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="911" class="logLine "><code><span>emoji       ┊ </span></code>
<br>
</span><span data-lineid="912" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.818s</span></code>
<br>
</span><span data-lineid="913" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="914" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.243s</span></code>
<br>
</span><span data-lineid="915" class="logLine "><code><span>emoji       ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 1.061s </span></code>
<br>
</span><span data-lineid="916" class="logLine "><code><span>emoji       ┊ </span></code>
<br>
</span><span data-lineid="917" class="logLine "><code><span>words       ┊ </span></code>
<br>
</span><span data-lineid="918" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">──┤ Building: </span><span>words</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="919" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [words]</span></code>
<br>
</span><span data-lineid="920" class="logLine "><code><span>words       ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="921" class="logLine "><code><span>words       ┊   FROM python:3.6</span></code>
<br>
</span><span data-lineid="922" class="logLine "><code><span>words       ┊   </span></code>
<br>
</span><span data-lineid="923" class="logLine "><code><span>words       ┊   ADD . /app</span></code>
<br>
</span><span data-lineid="924" class="logLine "><code><span>words       ┊   RUN cd /app &amp;&amp; pip install -r requirements.txt</span></code>
<br>
</span><span data-lineid="925" class="logLine "><code><span>words       ┊ </span></code>
<br>
</span><span data-lineid="926" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="927" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="928" class="logLine "><code><span>words       ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="929" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:33:58 Heartbeat</span></code>
<br>
</span><span data-lineid="930" class="logLine "><code><span>words       ┊     ╎ [1/3] FROM docker.io/library/python:3.6@sha256:52f872eae9755743c9494e0e3cf02a47d34b42032cab1e5ab777b30c3665d5f1</span></code>
<br>
</span><span data-lineid="931" class="logLine "><code><span>words       ┊     ╎ [cached] [2/3] ADD . /app</span></code>
<br>
</span><span data-lineid="932" class="logLine "><code><span>words       ┊     ╎ [cached] [3/3] RUN cd /app &amp;&amp; pip install -r requirements.txt</span></code>
<br>
</span><span data-lineid="933" class="logLine "><code><span>words       ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="934" class="logLine "><code><span>words       ┊ </span></code>
<br>
</span><span data-lineid="935" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/words:tilt-c44b006534a32c3e</span></code>
<br>
</span><span data-lineid="936" class="logLine "><code><span>words       ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="937" class="logLine "><code><span>words       ┊ </span></code>
<br>
</span><span data-lineid="938" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="939" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="940" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="941" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>   dan-words:deployment</span></code>
<br>
</span><span data-lineid="942" class="logLine "><code><span>words       ┊ </span></code>
<br>
</span><span data-lineid="943" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.540s</span></code>
<br>
</span><span data-lineid="944" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="945" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.186s</span></code>
<br>
</span><span data-lineid="946" class="logLine "><code><span>words       ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.727s </span></code>
<br>
</span><span data-lineid="947" class="logLine "><code><span>words       ┊ </span></code>
<br>
</span><span data-lineid="948" class="logLine "><code><span>random      ┊ </span></code>
<br>
</span><span data-lineid="949" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">──┤ Building: </span><span>random</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="950" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [random]</span></code>
<br>
</span><span data-lineid="951" class="logLine "><code><span>random      ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="952" class="logLine "><code><span>random      ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="953" class="logLine "><code><span>random      ┊   </span></code>
<br>
</span><span data-lineid="954" class="logLine "><code><span>random      ┊   ADD . /go/src/github.com/tilt-dev/servantes/random</span></code>
<br>
</span><span data-lineid="955" class="logLine "><code><span>random      ┊   RUN go install github.com/tilt-dev/servantes/random</span></code>
<br>
</span><span data-lineid="956" class="logLine "><code><span>random      ┊   </span></code>
<br>
</span><span data-lineid="957" class="logLine "><code><span>random      ┊   ENTRYPOINT /go/bin/random</span></code>
<br>
</span><span data-lineid="958" class="logLine "><code><span>random      ┊ </span></code>
<br>
</span><span data-lineid="959" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="960" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="961" class="logLine "><code><span>random      ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="962" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:33:59 Server status: All good</span></code>
<br>
</span><span data-lineid="963" class="logLine "><code><span>random      ┊     ╎ [1/3] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="964" class="logLine "><code><span>random      ┊     ╎ [cached] [2/3] ADD . /go/src/github.com/tilt-dev/servantes/random</span></code>
<br>
</span><span data-lineid="965" class="logLine "><code><span>random      ┊     ╎ [cached] [3/3] RUN go install github.com/tilt-dev/servantes/random</span></code>
<br>
</span><span data-lineid="966" class="logLine "><code><span>random      ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="967" class="logLine "><code><span>random      ┊ </span></code>
<br>
</span><span data-lineid="968" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/random:tilt-1aced68ac0fa0254</span></code>
<br>
</span><span data-lineid="969" class="logLine "><code><span>random      ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="970" class="logLine "><code><span>random      ┊ </span></code>
<br>
</span><span data-lineid="971" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="972" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="973" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="974" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>   dan-random:service</span></code>
<br>
</span><span data-lineid="975" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>   dan-random:deployment</span></code>
<br>
</span><span data-lineid="976" class="logLine "><code><span>random      ┊ </span></code>
<br>
</span><span data-lineid="977" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.519s</span></code>
<br>
</span><span data-lineid="978" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="979" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.209s</span></code>
<br>
</span><span data-lineid="980" class="logLine "><code><span>random      ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.728s </span></code>
<br>
</span><span data-lineid="981" class="logLine "><code><span>random      ┊ </span></code>
<br>
</span><span data-lineid="982" class="logLine "><code><span>secrets     ┊ </span></code>
<br>
</span><span data-lineid="983" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">──┤ Building: </span><span>secrets</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="984" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [secrets]</span></code>
<br>
</span><span data-lineid="985" class="logLine "><code><span>secrets     ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="986" class="logLine "><code><span>secrets     ┊   FROM golang:1.10</span></code>
<br>
</span><span data-lineid="987" class="logLine "><code><span>secrets     ┊   </span></code>
<br>
</span><span data-lineid="988" class="logLine "><code><span>secrets     ┊   ADD . /go/src/github.com/tilt-dev/servantes/secrets</span></code>
<br>
</span><span data-lineid="989" class="logLine "><code><span>secrets     ┊   RUN go install github.com/tilt-dev/servantes/secrets</span></code>
<br>
</span><span data-lineid="990" class="logLine "><code><span>secrets     ┊   </span></code>
<br>
</span><span data-lineid="991" class="logLine "><code><span>secrets     ┊   ENTRYPOINT /go/bin/secrets</span></code>
<br>
</span><span data-lineid="992" class="logLine "><code><span>secrets     ┊ </span></code>
<br>
</span><span data-lineid="993" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="994" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="995" class="logLine "><code><span>secrets     ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="996" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="997" class="logLine "><code><span>secrets     ┊     ╎ [1/3] FROM docker.io/library/golang:1.10@sha256:6d5e79878a3e4f1b30b7aa4d24fb6ee6184e905a9b172fc72593935633be4c46</span></code>
<br>
</span><span data-lineid="998" class="logLine "><code><span>secrets     ┊     ╎ [cached] [2/3] ADD . /go/src/github.com/tilt-dev/servantes/secrets</span></code>
<br>
</span><span data-lineid="999" class="logLine "><code><span>secrets     ┊     ╎ [cached] [3/3] RUN go install github.com/tilt-dev/servantes/secrets</span></code>
<br>
</span><span data-lineid="1000" class="logLine "><code><span>secrets     ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="1001" class="logLine "><code><span>secrets     ┊ </span></code>
<br>
</span><span data-lineid="1002" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/secrets:tilt-68091a75d1fd3000</span></code>
<br>
</span><span data-lineid="1003" class="logLine "><code><span>secrets     ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="1004" class="logLine "><code><span>secrets     ┊ </span></code>
<br>
</span><span data-lineid="1005" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="1006" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="1007" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="1008" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>   dan-secrets:service</span></code>
<br>
</span><span data-lineid="1009" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>   dan-secrets:deployment</span></code>
<br>
</span><span data-lineid="1010" class="logLine "><code><span>secrets     ┊ </span></code>
<br>
</span><span data-lineid="1011" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.522s</span></code>
<br>
</span><span data-lineid="1012" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="1013" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.195s</span></code>
<br>
</span><span data-lineid="1014" class="logLine "><code><span>secrets     ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.717s </span></code>
<br>
</span><span data-lineid="1015" class="logLine "><code><span>secrets     ┊ </span></code>
<br>
</span><span data-lineid="1016" class="logLine "><code><span>sleep       ┊ </span></code>
<br>
</span><span data-lineid="1017" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">──┤ Building: </span><span>sleep</span><span class="ansi-blue"> ├──────────────────────────────────────────────</span></code>
<br>
</span><span data-lineid="1018" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">STEP 1/3 — </span><span>Building Dockerfile: [sleep]</span></code>
<br>
</span><span data-lineid="1019" class="logLine "><code><span>sleep       ┊ Building Dockerfile:</span></code>
<br>
</span><span data-lineid="1020" class="logLine "><code><span>sleep       ┊   FROM node:10</span></code>
<br>
</span><span data-lineid="1021" class="logLine "><code><span>sleep       ┊   </span></code>
<br>
</span><span data-lineid="1022" class="logLine "><code><span>sleep       ┊   ADD . /</span></code>
<br>
</span><span data-lineid="1023" class="logLine "><code><span>sleep       ┊   </span></code>
<br>
</span><span data-lineid="1024" class="logLine "><code><span>sleep       ┊   ENTRYPOINT [ "node", "index.js" ]</span></code>
<br>
</span><span data-lineid="1025" class="logLine "><code><span>sleep       ┊ </span></code>
<br>
</span><span data-lineid="1026" class="logLine "><code><span>sleep       ┊ </span></code>
<br>
</span><span data-lineid="1027" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Tarring context…</span></code>
<br>
</span><span data-lineid="1028" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Building image</span></code>
<br>
</span><span data-lineid="1029" class="logLine "><code><span>sleep       ┊     ╎ copy /context /</span></code>
<br>
</span><span data-lineid="1030" class="logLine "><code><span>sleep       ┊     ╎ [1/2] FROM docker.io/library/node:10@sha256:dabc15ad36a9e0a95862fbdf6ffdad439edc20aa27c7f10456644464e3fb5f08</span></code>
<br>
</span><span data-lineid="1031" class="logLine "><code><span>sleep       ┊     ╎ [cached] [2/2] ADD . /</span></code>
<br>
</span><span data-lineid="1032" class="logLine "><code><span>sleep       ┊     ╎ exporting to image</span></code>
<br>
</span><span data-lineid="1033" class="logLine "><code><span>sleep       ┊ </span></code>
<br>
</span><span data-lineid="1034" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">STEP 2/3 — </span><span>Pushing gcr.io/windmill-public-containers/servantes/sleep:tilt-34ed95383c6f5b98</span></code>
<br>
</span><span data-lineid="1035" class="logLine "><code><span>sleep       ┊     ╎ Skipping push</span></code>
<br>
</span><span data-lineid="1036" class="logLine "><code><span>sleep       ┊ </span></code>
<br>
</span><span data-lineid="1037" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">STEP 3/3 — </span><span>Deploying</span></code>
<br>
</span><span data-lineid="1038" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Injecting images into Kubernetes YAML</span></code>
<br>
</span><span data-lineid="1039" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Applying via kubectl:</span></code>
<br>
</span><span data-lineid="1040" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>   sleep:pod</span></code>
<br>
</span><span data-lineid="1041" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1042" class="logLine "><code><span>sleep       ┊ </span></code>
<br>
</span><span data-lineid="1043" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Step 1 - 0.506s</span></code>
<br>
</span><span data-lineid="1044" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Step 2 - 0.000s</span></code>
<br>
</span><span data-lineid="1045" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Step 3 - 0.228s</span></code>
<br>
</span><span data-lineid="1046" class="logLine "><code><span>sleep       ┊ </span><span class="ansi-blue">  │ </span><span>Done in: 0.734s </span></code>
<br>
</span><span data-lineid="1047" class="logLine "><code><span>sleep       ┊ </span></code>
<br>
</span><span data-lineid="1048" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:01 Heartbeat</span></code>
<br>
</span><span data-lineid="1049" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1050" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1051" class="logLine "><code><span>sleep       ┊ Taking a break...</span></code>
<br>
</span><span data-lineid="1052" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1053" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:04 Heartbeat</span></code>
<br>
</span><span data-lineid="1054" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1055" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1056" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1057" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1058" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:08 Heartbeat</span></code>
<br>
</span><span data-lineid="1059" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1060" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1061" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1062" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:11 Heartbeat</span></code>
<br>
</span><span data-lineid="1063" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:34:10 UTC 2019</span></code>
<br>
</span><span data-lineid="1064" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1065" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1066" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1067" class="logLine "><code><span>sleep       ┊ Ten seconds later</span></code>
<br>
</span><span data-lineid="1068" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1069" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:14 Heartbeat</span></code>
<br>
</span><span data-lineid="1070" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1071" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1072" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1073" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:17 Heartbeat</span></code>
<br>
</span><span data-lineid="1074" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1075" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1076" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1077" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:20 Heartbeat</span></code>
<br>
</span><span data-lineid="1078" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1079" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1080" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1081" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1082" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:24 Heartbeat</span></code>
<br>
</span><span data-lineid="1083" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1084" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1085" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1086" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:27 Heartbeat</span></code>
<br>
</span><span data-lineid="1087" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1088" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1089" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1090" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:30 Heartbeat</span></code>
<br>
</span><span data-lineid="1091" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1092" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1093" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1094" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:33 Heartbeat</span></code>
<br>
</span><span data-lineid="1095" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1096" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1097" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1098" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:36 Heartbeat</span></code>
<br>
</span><span data-lineid="1099" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1100" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1101" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1102" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1103" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:40 Heartbeat</span></code>
<br>
</span><span data-lineid="1104" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1105" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1106" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1107" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:43 Heartbeat</span></code>
<br>
</span><span data-lineid="1108" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1109" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1110" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1111" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:46 Heartbeat</span></code>
<br>
</span><span data-lineid="1112" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1113" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1114" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1115" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:49 Heartbeat</span></code>
<br>
</span><span data-lineid="1116" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1117" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1118" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1119" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:52 Heartbeat</span></code>
<br>
</span><span data-lineid="1120" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1121" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1122" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1123" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1124" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:56 Heartbeat</span></code>
<br>
</span><span data-lineid="1125" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1126" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1127" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:34:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1128" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:34:59 Heartbeat</span></code>
<br>
</span><span data-lineid="1129" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1130" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1131" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1132" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:02 Heartbeat</span></code>
<br>
</span><span data-lineid="1133" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1134" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1135" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1136" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:05 Heartbeat</span></code>
<br>
</span><span data-lineid="1137" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1138" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1139" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1140" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:08 Heartbeat</span></code>
<br>
</span><span data-lineid="1141" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1142" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1143" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1144" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:35:10 UTC 2019</span></code>
<br>
</span><span data-lineid="1145" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1146" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1147" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:12 Heartbeat</span></code>
<br>
</span><span data-lineid="1148" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1149" class="logLine "><code><span>tick        ┊ [K8s EVENT: Pod tick-1573684500-lsltn (ns: default)] Failed create pod sandbox: rpc error: code = Unknown desc = failed to start sandbox container for pod "tick-1573684500-lsltn": Error response from daemon: OCI runtime start failed: container process is already dead: unknown</span></code>
<br>
</span><span data-lineid="1150" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1151" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1152" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:15 Heartbeat</span></code>
<br>
</span><span data-lineid="1153" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1154" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1155" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1156" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:18 Heartbeat</span></code>
<br>
</span><span data-lineid="1157" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1158" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1159" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1160" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:21 Heartbeat</span></code>
<br>
</span><span data-lineid="1161" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1162" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1163" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1164" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:24 Heartbeat</span></code>
<br>
</span><span data-lineid="1165" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1166" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1167" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1168" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1169" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:28 Heartbeat</span></code>
<br>
</span><span data-lineid="1170" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1171" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1172" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1173" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:31 Heartbeat</span></code>
<br>
</span><span data-lineid="1174" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1175" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1176" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1177" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:34 Heartbeat</span></code>
<br>
</span><span data-lineid="1178" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1179" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1180" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1181" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:37 Heartbeat</span></code>
<br>
</span><span data-lineid="1182" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1183" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1184" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1185" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:40 Heartbeat</span></code>
<br>
</span><span data-lineid="1186" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1187" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1188" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1189" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1190" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:44 Heartbeat</span></code>
<br>
</span><span data-lineid="1191" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1192" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1193" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1194" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:47 Heartbeat</span></code>
<br>
</span><span data-lineid="1195" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1196" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1197" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1198" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:50 Heartbeat</span></code>
<br>
</span><span data-lineid="1199" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1200" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1201" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1202" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:53 Heartbeat</span></code>
<br>
</span><span data-lineid="1203" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1204" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1205" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1206" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:35:56 Heartbeat</span></code>
<br>
</span><span data-lineid="1207" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1208" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1209" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:35:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1210" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1211" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:00 Heartbeat</span></code>
<br>
</span><span data-lineid="1212" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1213" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1214" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1215" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:03 Heartbeat</span></code>
<br>
</span><span data-lineid="1216" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1217" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1218" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1219" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:06 Heartbeat</span></code>
<br>
</span><span data-lineid="1220" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1221" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1222" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1223" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:09 Heartbeat</span></code>
<br>
</span><span data-lineid="1224" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1225" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1226" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1227" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:36:11 UTC 2019</span></code>
<br>
</span><span data-lineid="1228" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1229" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:12 Heartbeat</span></code>
<br>
</span><span data-lineid="1230" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1231" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1232" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1233" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1234" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:16 Heartbeat</span></code>
<br>
</span><span data-lineid="1235" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1236" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1237" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1238" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:19 Heartbeat</span></code>
<br>
</span><span data-lineid="1239" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1240" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1241" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1242" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:22 Heartbeat</span></code>
<br>
</span><span data-lineid="1243" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1244" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1245" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1246" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:25 Heartbeat</span></code>
<br>
</span><span data-lineid="1247" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1248" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1249" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1250" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:28 Heartbeat</span></code>
<br>
</span><span data-lineid="1251" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1252" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1253" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1254" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1255" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:32 Heartbeat</span></code>
<br>
</span><span data-lineid="1256" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1257" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1258" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1259" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:35 Heartbeat</span></code>
<br>
</span><span data-lineid="1260" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1261" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1262" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1263" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:38 Heartbeat</span></code>
<br>
</span><span data-lineid="1264" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1265" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1266" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1267" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:41 Heartbeat</span></code>
<br>
</span><span data-lineid="1268" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1269" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1270" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1271" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:44 Heartbeat</span></code>
<br>
</span><span data-lineid="1272" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1273" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1274" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1275" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1276" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:48 Heartbeat</span></code>
<br>
</span><span data-lineid="1277" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1278" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1279" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1280" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:51 Heartbeat</span></code>
<br>
</span><span data-lineid="1281" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1282" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1283" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1284" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:54 Heartbeat</span></code>
<br>
</span><span data-lineid="1285" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1286" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1287" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1288" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:36:57 Heartbeat</span></code>
<br>
</span><span data-lineid="1289" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1290" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:36:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1291" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1292" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:00 Heartbeat</span></code>
<br>
</span><span data-lineid="1293" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1294" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1295" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1296" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1297" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:04 Heartbeat</span></code>
<br>
</span><span data-lineid="1298" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1299" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1300" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1301" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:07 Heartbeat</span></code>
<br>
</span><span data-lineid="1302" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1303" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1304" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1305" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:10 Heartbeat</span></code>
<br>
</span><span data-lineid="1306" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1307" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:37:10 UTC 2019</span></code>
<br>
</span><span data-lineid="1308" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1309" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1310" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1311" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:13 Heartbeat</span></code>
<br>
</span><span data-lineid="1312" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1313" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1314" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1315" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:16 Heartbeat</span></code>
<br>
</span><span data-lineid="1316" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1317" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1318" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1319" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1320" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:20 Heartbeat</span></code>
<br>
</span><span data-lineid="1321" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1322" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1323" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1324" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:23 Heartbeat</span></code>
<br>
</span><span data-lineid="1325" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1326" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1327" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1328" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:26 Heartbeat</span></code>
<br>
</span><span data-lineid="1329" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1330" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1331" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1332" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:29 Heartbeat</span></code>
<br>
</span><span data-lineid="1333" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1334" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1335" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1336" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:32 Heartbeat</span></code>
<br>
</span><span data-lineid="1337" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1338" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1339" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1340" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1341" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:36 Heartbeat</span></code>
<br>
</span><span data-lineid="1342" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1343" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1344" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1345" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:39 Heartbeat</span></code>
<br>
</span><span data-lineid="1346" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1347" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1348" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1349" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:42 Heartbeat</span></code>
<br>
</span><span data-lineid="1350" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1351" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1352" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1353" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:45 Heartbeat</span></code>
<br>
</span><span data-lineid="1354" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1355" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1356" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1357" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:48 Heartbeat</span></code>
<br>
</span><span data-lineid="1358" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1359" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1360" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1361" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1362" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:52 Heartbeat</span></code>
<br>
</span><span data-lineid="1363" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1364" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1365" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1366" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:55 Heartbeat</span></code>
<br>
</span><span data-lineid="1367" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1368" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1369" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1370" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:37:58 Heartbeat</span></code>
<br>
</span><span data-lineid="1371" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:37:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1372" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1373" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1374" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:01 Heartbeat</span></code>
<br>
</span><span data-lineid="1375" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1376" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1377" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1378" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:04 Heartbeat</span></code>
<br>
</span><span data-lineid="1379" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1380" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1381" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1382" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1383" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:08 Heartbeat</span></code>
<br>
</span><span data-lineid="1384" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1385" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1386" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1387" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:11 Heartbeat</span></code>
<br>
</span><span data-lineid="1388" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:38:11 UTC 2019</span></code>
<br>
</span><span data-lineid="1389" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1390" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1391" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:38:11 UTC 2019</span></code>
<br>
</span><span data-lineid="1392" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1393" class="logLine "><code><span>tick        ┊ [K8s EVENT: Pod tick-1573684680-2d9r5 (ns: default)] Failed create pod sandbox: rpc error: code = Unknown desc = failed to start sandbox container for pod "tick-1573684680-2d9r5": Error response from daemon: OCI runtime create failed: container_linux.go:346: starting container process caused "process_linux.go:319: getting the final child's pid from pipe caused "read init-p: connection reset by peer"": unknown</span></code>
<br>
</span><span data-lineid="1394" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1395" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1396" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:14 Heartbeat</span></code>
<br>
</span><span data-lineid="1397" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1398" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1399" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1400" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:17 Heartbeat</span></code>
<br>
</span><span data-lineid="1401" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1402" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1403" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1404" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:20 Heartbeat</span></code>
<br>
</span><span data-lineid="1405" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1406" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1407" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1408" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1409" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:24 Heartbeat</span></code>
<br>
</span><span data-lineid="1410" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1411" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1412" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1413" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:27 Heartbeat</span></code>
<br>
</span><span data-lineid="1414" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1415" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1416" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1417" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:30 Heartbeat</span></code>
<br>
</span><span data-lineid="1418" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1419" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1420" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1421" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:33 Heartbeat</span></code>
<br>
</span><span data-lineid="1422" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1423" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1424" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1425" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:36 Heartbeat</span></code>
<br>
</span><span data-lineid="1426" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1427" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1428" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1429" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1430" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:40 Heartbeat</span></code>
<br>
</span><span data-lineid="1431" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1432" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1433" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1434" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:43 Heartbeat</span></code>
<br>
</span><span data-lineid="1435" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1436" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1437" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1438" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:46 Heartbeat</span></code>
<br>
</span><span data-lineid="1439" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1440" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1441" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1442" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:49 Heartbeat</span></code>
<br>
</span><span data-lineid="1443" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1444" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1445" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1446" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:52 Heartbeat</span></code>
<br>
</span><span data-lineid="1447" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1448" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1449" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1450" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1451" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:56 Heartbeat</span></code>
<br>
</span><span data-lineid="1452" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1453" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1454" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:38:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1455" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:38:59 Heartbeat</span></code>
<br>
</span><span data-lineid="1456" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1457" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1458" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1459" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:02 Heartbeat</span></code>
<br>
</span><span data-lineid="1460" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1461" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1462" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1463" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:05 Heartbeat</span></code>
<br>
</span><span data-lineid="1464" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1465" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1466" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1467" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:08 Heartbeat</span></code>
<br>
</span><span data-lineid="1468" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1469" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1470" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1471" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1472" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:12 Heartbeat</span></code>
<br>
</span><span data-lineid="1473" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:39:11 UTC 2019</span></code>
<br>
</span><span data-lineid="1474" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1475" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1476" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1477" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1478" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:15 Heartbeat</span></code>
<br>
</span><span data-lineid="1479" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1480" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1481" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1482" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:18 Heartbeat</span></code>
<br>
</span><span data-lineid="1483" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1484" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1485" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1486" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:21 Heartbeat</span></code>
<br>
</span><span data-lineid="1487" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1488" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1489" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1490" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:24 Heartbeat</span></code>
<br>
</span><span data-lineid="1491" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1492" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1493" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1494" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1495" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:28 Heartbeat</span></code>
<br>
</span><span data-lineid="1496" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1497" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1498" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1499" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:31 Heartbeat</span></code>
<br>
</span><span data-lineid="1500" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1501" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1502" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1503" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:34 Heartbeat</span></code>
<br>
</span><span data-lineid="1504" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1505" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1506" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1507" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:37 Heartbeat</span></code>
<br>
</span><span data-lineid="1508" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1509" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1510" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1511" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:40 Heartbeat</span></code>
<br>
</span><span data-lineid="1512" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1513" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1514" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1515" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1516" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:44 Heartbeat</span></code>
<br>
</span><span data-lineid="1517" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1518" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1519" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1520" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:47 Heartbeat</span></code>
<br>
</span><span data-lineid="1521" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1522" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1523" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1524" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:50 Heartbeat</span></code>
<br>
</span><span data-lineid="1525" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1526" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1527" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1528" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:53 Heartbeat</span></code>
<br>
</span><span data-lineid="1529" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1530" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1531" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1532" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:39:56 Heartbeat</span></code>
<br>
</span><span data-lineid="1533" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1534" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1535" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:39:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1536" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1537" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:00 Heartbeat</span></code>
<br>
</span><span data-lineid="1538" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1539" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1540" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:02 Server status: All good</span></code>
<br>
</span><span data-lineid="1541" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:03 Heartbeat</span></code>
<br>
</span><span data-lineid="1542" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1543" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:04 Server status: All good</span></code>
<br>
</span><span data-lineid="1544" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1545" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:06 Heartbeat</span></code>
<br>
</span><span data-lineid="1546" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:06 Server status: All good</span></code>
<br>
</span><span data-lineid="1547" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1548" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:08 Server status: All good</span></code>
<br>
</span><span data-lineid="1549" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:09 Heartbeat</span></code>
<br>
</span><span data-lineid="1550" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1551" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:10 Server status: All good</span></code>
<br>
</span><span data-lineid="1552" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:40:11 UTC 2019</span></code>
<br>
</span><span data-lineid="1553" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1554" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1555" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:40:11 UTC 2019</span></code>
<br>
</span><span data-lineid="1556" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1557" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:12 Heartbeat</span></code>
<br>
</span><span data-lineid="1558" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:12 Server status: All good</span></code>
<br>
</span><span data-lineid="1559" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1560" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:14 Server status: All good</span></code>
<br>
</span><span data-lineid="1561" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1562" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:16 Heartbeat</span></code>
<br>
</span><span data-lineid="1563" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:16 Server status: All good</span></code>
<br>
</span><span data-lineid="1564" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1565" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:18 Server status: All good</span></code>
<br>
</span><span data-lineid="1566" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:19 Heartbeat</span></code>
<br>
</span><span data-lineid="1567" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1568" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1569" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1570" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:22 Heartbeat</span></code>
<br>
</span><span data-lineid="1571" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1572" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1573" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1574" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:25 Heartbeat</span></code>
<br>
</span><span data-lineid="1575" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1576" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1577" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1578" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:28 Heartbeat</span></code>
<br>
</span><span data-lineid="1579" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1580" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1581" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1582" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1583" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:32 Heartbeat</span></code>
<br>
</span><span data-lineid="1584" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1585" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1586" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:35 Heartbeat</span></code>
<br>
</span><span data-lineid="1587" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1588" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1589" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1590" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1591" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:38 Heartbeat</span></code>
<br>
</span><span data-lineid="1592" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1593" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1594" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1595" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:41 Heartbeat</span></code>
<br>
</span><span data-lineid="1596" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1597" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1598" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1599" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:44 Heartbeat</span></code>
<br>
</span><span data-lineid="1600" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1601" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1602" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1603" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1604" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:48 Heartbeat</span></code>
<br>
</span><span data-lineid="1605" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1606" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1607" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:51 Heartbeat</span></code>
<br>
</span><span data-lineid="1608" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1609" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1610" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1611" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1612" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:54 Heartbeat</span></code>
<br>
</span><span data-lineid="1613" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1614" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1615" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1616" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:40:57 Heartbeat</span></code>
<br>
</span><span data-lineid="1617" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1618" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:40:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1619" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1620" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:00 Heartbeat</span></code>
<br>
</span><span data-lineid="1621" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1622" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1623" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1624" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1625" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:04 Heartbeat</span></code>
<br>
</span><span data-lineid="1626" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1627" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1628" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:07 Heartbeat</span></code>
<br>
</span><span data-lineid="1629" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1630" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1631" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1632" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1633" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:10 Heartbeat</span></code>
<br>
</span><span data-lineid="1634" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1635" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1636" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:41:11 UTC 2019</span></code>
<br>
</span><span data-lineid="1637" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1638" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1639" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:13 Heartbeat</span></code>
<br>
</span><span data-lineid="1640" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1641" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1642" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1643" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:16 Heartbeat</span></code>
<br>
</span><span data-lineid="1644" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1645" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1646" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1647" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1648" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:20 Heartbeat</span></code>
<br>
</span><span data-lineid="1649" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1650" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1651" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:23 Heartbeat</span></code>
<br>
</span><span data-lineid="1652" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1653" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1654" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1655" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1656" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:26 Heartbeat</span></code>
<br>
</span><span data-lineid="1657" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1658" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1659" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1660" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:29 Heartbeat</span></code>
<br>
</span><span data-lineid="1661" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1662" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1663" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1664" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:32 Heartbeat</span></code>
<br>
</span><span data-lineid="1665" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1666" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1667" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1668" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1669" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:36 Heartbeat</span></code>
<br>
</span><span data-lineid="1670" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1671" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1672" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:39 Heartbeat</span></code>
<br>
</span><span data-lineid="1673" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1674" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1675" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1676" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1677" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:42 Heartbeat</span></code>
<br>
</span><span data-lineid="1678" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1679" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1680" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1681" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:45 Heartbeat</span></code>
<br>
</span><span data-lineid="1682" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1683" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1684" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1685" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:48 Heartbeat</span></code>
<br>
</span><span data-lineid="1686" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1687" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1688" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1689" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1690" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:52 Heartbeat</span></code>
<br>
</span><span data-lineid="1691" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1692" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1693" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:55 Heartbeat</span></code>
<br>
</span><span data-lineid="1694" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1695" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1696" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1697" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1698" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:41:58 Heartbeat</span></code>
<br>
</span><span data-lineid="1699" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:41:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1700" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1701" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1702" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:01 Heartbeat</span></code>
<br>
</span><span data-lineid="1703" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1704" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:42:01 UTC 2019</span></code>
<br>
</span><span data-lineid="1705" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1706" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1707" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1708" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:04 Heartbeat</span></code>
<br>
</span><span data-lineid="1709" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1710" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1711" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1712" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1713" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:08 Heartbeat</span></code>
<br>
</span><span data-lineid="1714" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1715" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1716" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:11 Heartbeat</span></code>
<br>
</span><span data-lineid="1717" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1718" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1719" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1720" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1721" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:14 Heartbeat</span></code>
<br>
</span><span data-lineid="1722" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1723" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1724" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1725" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:17 Heartbeat</span></code>
<br>
</span><span data-lineid="1726" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1727" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1728" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1729" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:20 Heartbeat</span></code>
<br>
</span><span data-lineid="1730" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1731" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1732" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1733" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1734" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:24 Heartbeat</span></code>
<br>
</span><span data-lineid="1735" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1736" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1737" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:27 Heartbeat</span></code>
<br>
</span><span data-lineid="1738" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1739" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1740" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1741" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1742" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:30 Heartbeat</span></code>
<br>
</span><span data-lineid="1743" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1744" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1745" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1746" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:33 Heartbeat</span></code>
<br>
</span><span data-lineid="1747" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1748" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1749" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1750" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:36 Heartbeat</span></code>
<br>
</span><span data-lineid="1751" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1752" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1753" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1754" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1755" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:40 Heartbeat</span></code>
<br>
</span><span data-lineid="1756" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1757" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1758" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:43 Heartbeat</span></code>
<br>
</span><span data-lineid="1759" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1760" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1761" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1762" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1763" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:46 Heartbeat</span></code>
<br>
</span><span data-lineid="1764" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1765" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1766" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1767" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:49 Heartbeat</span></code>
<br>
</span><span data-lineid="1768" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1769" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1770" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1771" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:52 Heartbeat</span></code>
<br>
</span><span data-lineid="1772" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1773" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1774" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1775" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1776" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:56 Heartbeat</span></code>
<br>
</span><span data-lineid="1777" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1778" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1779" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:42:59 Heartbeat</span></code>
<br>
</span><span data-lineid="1780" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:42:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1781" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1782" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1783" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1784" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:43:01 UTC 2019</span></code>
<br>
</span><span data-lineid="1785" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1786" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:02 Heartbeat</span></code>
<br>
</span><span data-lineid="1787" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1788" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1789" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1790" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:05 Heartbeat</span></code>
<br>
</span><span data-lineid="1791" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1792" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1793" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1794" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:08 Heartbeat</span></code>
<br>
</span><span data-lineid="1795" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1796" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1797" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1798" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1799" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:12 Heartbeat</span></code>
<br>
</span><span data-lineid="1800" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1801" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1802" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:15 Heartbeat</span></code>
<br>
</span><span data-lineid="1803" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1804" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1805" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1806" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1807" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:18 Heartbeat</span></code>
<br>
</span><span data-lineid="1808" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1809" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1810" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1811" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:21 Heartbeat</span></code>
<br>
</span><span data-lineid="1812" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1813" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1814" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1815" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:24 Heartbeat</span></code>
<br>
</span><span data-lineid="1816" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1817" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1818" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1819" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1820" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:28 Heartbeat</span></code>
<br>
</span><span data-lineid="1821" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1822" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1823" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:31 Heartbeat</span></code>
<br>
</span><span data-lineid="1824" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1825" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1826" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1827" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1828" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:34 Heartbeat</span></code>
<br>
</span><span data-lineid="1829" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1830" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1831" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1832" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:37 Heartbeat</span></code>
<br>
</span><span data-lineid="1833" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1834" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1835" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1836" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:40 Heartbeat</span></code>
<br>
</span><span data-lineid="1837" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1838" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1839" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1840" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1841" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:44 Heartbeat</span></code>
<br>
</span><span data-lineid="1842" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1843" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1844" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:47 Heartbeat</span></code>
<br>
</span><span data-lineid="1845" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:47 Server status: All good</span></code>
<br>
</span><span data-lineid="1846" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1847" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:49 Server status: All good</span></code>
<br>
</span><span data-lineid="1848" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1849" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:50 Heartbeat</span></code>
<br>
</span><span data-lineid="1850" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:51 Server status: All good</span></code>
<br>
</span><span data-lineid="1851" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1852" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:53 Server status: All good</span></code>
<br>
</span><span data-lineid="1853" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:53 Heartbeat</span></code>
<br>
</span><span data-lineid="1854" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1855" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:55 Server status: All good</span></code>
<br>
</span><span data-lineid="1856" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1857" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:43:56 Heartbeat</span></code>
<br>
</span><span data-lineid="1858" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:57 Server status: All good</span></code>
<br>
</span><span data-lineid="1859" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1860" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:43:59 Server status: All good</span></code>
<br>
</span><span data-lineid="1861" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1862" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:00 Heartbeat</span></code>
<br>
</span><span data-lineid="1863" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:01 Server status: All good</span></code>
<br>
</span><span data-lineid="1864" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1865" class="logLine "><code><span>tick        ┊ Wed Nov 13 22:44:01 UTC 2019</span></code>
<br>
</span><span data-lineid="1866" class="logLine "><code><span>tick        ┊ tick</span></code>
<br>
</span><span data-lineid="1867" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:03 Heartbeat</span></code>
<br>
</span><span data-lineid="1868" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:03 Server status: All good</span></code>
<br>
</span><span data-lineid="1869" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1870" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:05 Server status: All good</span></code>
<br>
</span><span data-lineid="1871" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1872" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:06 Heartbeat</span></code>
<br>
</span><span data-lineid="1873" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:07 Server status: All good</span></code>
<br>
</span><span data-lineid="1874" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1875" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:09 Server status: All good</span></code>
<br>
</span><span data-lineid="1876" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:09 Heartbeat</span></code>
<br>
</span><span data-lineid="1877" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1878" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:11 Server status: All good</span></code>
<br>
</span><span data-lineid="1879" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1880" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:12 Heartbeat</span></code>
<br>
</span><span data-lineid="1881" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:13 Server status: All good</span></code>
<br>
</span><span data-lineid="1882" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1883" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:15 Server status: All good</span></code>
<br>
</span><span data-lineid="1884" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1885" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:15 Heartbeat</span></code>
<br>
</span><span data-lineid="1886" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:17 Server status: All good</span></code>
<br>
</span><span data-lineid="1887" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1888" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:19 Heartbeat</span></code>
<br>
</span><span data-lineid="1889" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:19 Server status: All good</span></code>
<br>
</span><span data-lineid="1890" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1891" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:21 Server status: All good</span></code>
<br>
</span><span data-lineid="1892" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1893" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:22 Heartbeat</span></code>
<br>
</span><span data-lineid="1894" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:23 Server status: All good</span></code>
<br>
</span><span data-lineid="1895" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1896" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:25 Server status: All good</span></code>
<br>
</span><span data-lineid="1897" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:25 Heartbeat</span></code>
<br>
</span><span data-lineid="1898" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1899" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:27 Server status: All good</span></code>
<br>
</span><span data-lineid="1900" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1901" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:28 Heartbeat</span></code>
<br>
</span><span data-lineid="1902" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:29 Server status: All good</span></code>
<br>
</span><span data-lineid="1903" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1904" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:31 Server status: All good</span></code>
<br>
</span><span data-lineid="1905" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1906" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:31 Heartbeat</span></code>
<br>
</span><span data-lineid="1907" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:33 Server status: All good</span></code>
<br>
</span><span data-lineid="1908" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1909" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:35 Heartbeat</span></code>
<br>
</span><span data-lineid="1910" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:35 Server status: All good</span></code>
<br>
</span><span data-lineid="1911" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1912" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:37 Server status: All good</span></code>
<br>
</span><span data-lineid="1913" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1914" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:38 Heartbeat</span></code>
<br>
</span><span data-lineid="1915" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:39 Server status: All good</span></code>
<br>
</span><span data-lineid="1916" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1917" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:41 Server status: All good</span></code>
<br>
</span><span data-lineid="1918" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:41 Heartbeat</span></code>
<br>
</span><span data-lineid="1919" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span id="start1" data-lineid="1920" class="logLine "><code><span id="start2">vigoda      ┊ 2019/11/13 22:44:43 Server status: All good</span></code>
<br>
</span><span data-lineid="1921" class="logLine "><code><span>doggos      ┊ [sidecar] I'm a loud sidecar!</span></code>
<br>
</span><span data-lineid="1922" class="logLine "><code><span>doggos      ┊ [doggos] 2019/11/13 22:44:44 Heartbeat</span></code>
<br>
</span><span id="end1" data-lineid="1923" class="logLine "><code><span>vigoda      ┊ 2019/11/13 22:44:45 Server status: All good</span></code>
<br>
</span><span data-lineid="1924" class="logLine "><code></code><br></span>
<p class="logEnd">█</p>
</section>`

export {
  oneResource,
  oneResourceView,
  twoResourceView,
  getMockRouterProps,
  oneResourceFailedToBuild,
  oneResourceCrashedOnStart,
  oneResourceBuilding,
  oneResourceNoAlerts,
  oneResourceImagePullBackOff,
  oneResourceErrImgPull,
  oneResourceUnrecognizedError,
  logPaneDOM,
  unnamedEndpointLink,
  namedEndpointLink,
}
