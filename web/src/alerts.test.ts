import {
  Alert,
  BuildFailedErrorType,
  CrashRebuildErrorType,
  getResourceAlerts,
  numberOfAlerts,
  PodRestartErrorType,
  PodStatusErrorType,
  WarningErrorType,
} from "./alerts"
import { TriggerMode } from "./types"
import LogStore from "./LogStore"

type Resource = Proto.webviewResource
type K8sResourceInfo = Proto.webviewK8sResourceInfo

let logStore = new LogStore()

beforeEach(() => {
  logStore = new LogStore()
})

describe("getResourceAlerts", () => {
  it("K8Resource: shows that a pod status of error is an alert", () => {
    let r: Resource = k8sResource()
    let rInfo = <K8sResourceInfo>r.k8sResourceInfo
    rInfo.podStatus = "Error"
    rInfo.podStatusMessage = "I'm a pod in Error"

    let actual = getResourceAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        alertType: PodStatusErrorType,
        msg: "I'm a pod in Error",
        timestamp: "",
        header: "",
        resourceName: "snack",
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s Resource: should show pod restart alert ", () => {
    let r: Resource = k8sResource()
    let rInfo = r.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podRestarts = 1
    let actual = getResourceAlerts(r, logStore).map(a => {
      delete a.dismissHandler
      return a
    })
    let expectedAlerts: Alert[] = [
      {
        alertType: PodRestartErrorType,
        msg: "",
        timestamp: "",
        header: "Restarts: 1",
        resourceName: "snack",
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s Resource should show the first build alert", () => {
    let r: Resource = k8sResource()
    r.buildHistory = [
      {
        finishTime: "10:00AM",
        error: "build failed",
        spanId: "build:1",
      },
      {},
    ]
    logStore.append({
      spans: { "build:1": r.name },
      segments: [{ text: "Build error log", spanId: "build:1" }],
    })
    let actual = getResourceAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        alertType: BuildFailedErrorType,
        msg: "Build error log",
        timestamp: "10:00AM",
        header: "Build error",
        resourceName: "snack",
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s Resource: should show a crash rebuild alert  using the first build info", () => {
    let r: Resource = k8sResource()
    r.crashLog = "Hello I am a crash log"
    r.buildHistory = [
      {
        isCrashRebuild: true,
      },
      {
        isCrashRebuild: true,
      },
    ]
    let rInfo = r.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podCreationTime = "10:00AM"

    let actual = getResourceAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        alertType: CrashRebuildErrorType,
        msg: "Hello I am a crash log",
        timestamp: "10:00AM",
        header: "Pod crashed",
        resourceName: "snack",
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s Resource: should show a warning alert using the first build history ", () => {
    let r: Resource = k8sResource()
    r.buildHistory = [
      {
        warnings: ["Hi i'm a warning"],
        finishTime: "10:00am",
      },
      {
        warnings: ["This warning shouldn't show up", "Or this one"],
      },
    ]

    let actual = getResourceAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        alertType: WarningErrorType,
        msg: "Hi i'm a warning",
        timestamp: "10:00am",
        header: "snack",
        resourceName: "snack",
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s Resource: should show a pod restart alert and a build failed alert", () => {
    let r: Resource = k8sResource()
    let rInfo = r.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podRestarts = 1 // triggers pod restart alert
    rInfo.podCreationTime = "10:00AM"

    r.crashLog = "I'm a pod that crashed"
    r.buildHistory = [
      // triggers build failed alert
      {
        finishTime: "10:00AM",
        error: "build failed",
        spanId: "build:1",
      },
      {},
    ]
    logStore.append({
      spans: { "build:1": r.name },
      segments: [{ text: "Build error log", spanId: "build:1" }],
    })

    let actual = getResourceAlerts(r, logStore).map(a => {
      delete a.dismissHandler
      return a
    })
    let expectedAlerts: Alert[] = [
      {
        alertType: PodRestartErrorType,
        msg: "I'm a pod that crashed",
        timestamp: "10:00AM",
        header: "Restarts: 1",
        resourceName: "snack",
      },
      {
        alertType: BuildFailedErrorType,
        msg: "Build error log",
        timestamp: "10:00AM",
        header: "Build error",
        resourceName: "snack",
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s Resource: should show 3 alerts: 1 crash rebuild alert, 1 build failed alert, 1 warning alert ", () => {
    let r: Resource = k8sResource()
    r.crashLog = "Hello I am a crash log"
    r.buildHistory = [
      {
        isCrashRebuild: true,
        warnings: ["Hi I am a warning"],
        finishTime: "10:00am",
        error: "build failed",
        spanId: "build:1",
      },
    ]
    logStore.append({
      spans: { "build:1": r.name },
      segments: [{ text: "Build failed log", spanId: "build:1" }],
    })
    let actual = getResourceAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        alertType: CrashRebuildErrorType,
        msg: "Hello I am a crash log",
        timestamp: "",
        header: "Pod crashed",
        resourceName: "snack",
      },
      {
        alertType: BuildFailedErrorType,
        msg: "Build failed log",
        timestamp: "10:00am",
        header: "Build error",
        resourceName: "snack",
      },
      {
        alertType: WarningErrorType,
        msg: "Hi I am a warning",
        timestamp: "10:00am",
        header: "snack",
        resourceName: "snack",
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s Resource: should show number of alerts a resource has", () => {
    let r: Resource = k8sResource()
    let rInfo = r.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podRestarts = 1
    let actualNum = numberOfAlerts(r)
    let expectedNum = 1

    expect(actualNum).toEqual(expectedNum)
  })
})

//DC Resource Tests
it("DC Resource: should show a warning alert using the first build history", () => {
  let r: Resource = dcResource()
  r.buildHistory = [
    {
      warnings: ["Hi i'm a warning"],
      finishTime: "10:00am",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let actual = getResourceAlerts(r, logStore)
  let expectedAlerts: Alert[] = [
    {
      alertType: WarningErrorType,
      msg: "Hi i'm a warning",
      timestamp: "10:00am",
      header: "vigoda",
      resourceName: "vigoda",
    },
  ]
  expect(actual).toEqual(expectedAlerts)
})

//DC Resource Tests
it("DC resource: should show a warning alert using the first build history", () => {
  let r: Resource = dcResource()
  r.buildHistory = [
    {
      warnings: ["Hi i'm a warning"],
      finishTime: "10:00am",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let actual = getResourceAlerts(r, logStore)
  let expectedAlerts: Alert[] = [
    {
      alertType: WarningErrorType,
      msg: "Hi i'm a warning",
      timestamp: "10:00am",
      header: "vigoda",
      resourceName: "vigoda",
    },
  ]
  expect(actual).toEqual(expectedAlerts)
})

it("DC Resource has build failed alert using first build history info ", () => {
  let r: Resource = dcResource()
  r.buildHistory = [
    {
      spanId: "build:1",
      error: "theres an error !!!!",
      finishTime: "10:00am",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  logStore.append({
    spans: { "build:1": r.name },
    segments: [{ text: "Hi you're build failed :'(", spanId: "build:1" }],
  })
  let actual = getResourceAlerts(r, logStore)
  let expectedAlerts: Alert[] = [
    {
      alertType: BuildFailedErrorType,
      msg: "Hi you're build failed :'(",
      timestamp: "10:00am",
      header: "Build error",
      resourceName: "vigoda",
    },
  ]
  expect(actual).toEqual(expectedAlerts)
})

it("renders a build error for both a K8s resource and DC resource ", () => {
  let dcresource: Resource = dcResource()
  dcresource.buildHistory = [
    {
      error: "theres an error !!!!",
      finishTime: "10:00am",
      spanId: "build:1",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let k8sresource: Resource = k8sResource()
  k8sresource.buildHistory = [
    {
      error: "theres an error !!!!",
      finishTime: "10:00am",
      spanId: "build:2",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  logStore.append({
    spans: {
      "build:1": { manifestName: dcresource.name },
      "build:2": { manifestName: k8sresource.name },
    },
    segments: [
      { text: "Hi your build failed :'(", spanId: "build:1" },
      { text: "Hi this (k8s) resource failed too :)", spanId: "build:2" },
    ],
  })

  let dcAlerts = getResourceAlerts(dcresource, logStore)
  let k8sAlerts = getResourceAlerts(k8sresource, logStore)

  let actual = dcAlerts.concat(k8sAlerts)
  let expectedAlerts: Alert[] = [
    {
      alertType: BuildFailedErrorType,
      msg: "Hi your build failed :'(",
      timestamp: "10:00am",
      header: "Build error",
      resourceName: "vigoda",
    },
    {
      alertType: BuildFailedErrorType,
      msg: "Hi this (k8s) resource failed too :)",
      timestamp: "10:00am",
      header: "Build error",
      resourceName: "snack",
    },
  ]
  expect(actual).toEqual(expectedAlerts)
})

function k8sResource(): Resource {
  return {
    name: "snack",
    buildHistory: [],
    crashLog: "",
    directoriesWatched: [],
    endpoints: [],
    podID: "podID",
    isTiltfile: false,
    lastDeployTime: "",
    pathsWatched: [],
    pendingBuildEdits: [],
    pendingBuildReason: 0,
    pendingBuildSince: "",
    k8sResourceInfo: {
      podName: "testPod",
      podCreationTime: "",
      podUpdateStartTime: "",
      podStatus: "",
      podStatusMessage: "",
      podRestarts: 0,
    },
    runtimeStatus: "",
    triggerMode: TriggerMode.TriggerModeAuto,
    hasPendingChanges: true,
    facets: [],
    queued: false,
  }
}

function dcResource(): Resource {
  return {
    name: "vigoda",
    directoriesWatched: [],
    pathsWatched: ["Tiltfile.dc"],
    lastDeployTime: "2019-08-07T11:43:37.568629-04:00",
    triggerMode: 0,
    buildHistory: [
      {
        startTime: "2019-08-07T11:43:32.422237-04:00",
        finishTime: "2019-08-07T11:43:37.568626-04:00",
        isCrashRebuild: false,
      },
    ],
    currentBuild: {
      startTime: "0001-01-01T00:00:00Z",
      finishTime: "0001-01-01T00:00:00Z",
      isCrashRebuild: false,
    },
    pendingBuildReason: 0,
    pendingBuildEdits: [],
    pendingBuildSince: "0001-01-01T00:00:00Z",
    hasPendingChanges: false,
    endpoints: ["http://localhost:9007/"],
    podID: "",
    dcResourceInfo: {
      configPaths: [""],
      containerStatus: "OK",
      containerID: "",
      startTime: "2019-08-07T11:43:36.900841-04:00",
    },
    runtimeStatus: "ok",
    isTiltfile: false,
    crashLog: "",
    facets: [],
    queued: false,
  }
}
