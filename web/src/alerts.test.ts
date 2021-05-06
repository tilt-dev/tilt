import { Alert, combinedAlerts } from "./alerts"
import { FilterLevel, FilterSource } from "./logfilters"
import LogStore from "./LogStore"
import { appendLinesForManifestAndSpan } from "./testlogs"
import { TriggerMode } from "./types"

type UIResource = Proto.v1alpha1UIResource
type K8sResourceInfo = Proto.v1alpha1UIResourceKubernetes

let logStore = new LogStore()

beforeEach(() => {
  logStore = new LogStore()
})

describe("combinedAlerts", () => {
  it("K8Resource: shows that a pod status of error is an alert", () => {
    let r: UIResource = k8sResource()
    let rInfo = <K8sResourceInfo>r.status!.k8sResourceInfo
    rInfo.podStatus = "Error"
    rInfo.podStatusMessage = "I'm a pod in Error"

    let actual = combinedAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        msg: "I'm a pod in Error",
        resourceName: "snack",
        level: FilterLevel.error,
        source: FilterSource.runtime,
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s UIResource: should show pod restart alert ", () => {
    let r: UIResource = k8sResource()
    let rInfo = r.status!.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podRestarts = 1
    let actual = combinedAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        msg: "Restarts: 1",
        resourceName: "snack",
        level: FilterLevel.warn,
        source: FilterSource.runtime,
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s UIResource should show the first build alert", () => {
    let r: UIResource = k8sResource()
    r.status!.buildHistory = [
      {
        finishTime: "10:00AM",
        error: "build failed",
        spanID: "build:1",
      },
      {},
    ]
    logStore.append({
      spans: { "build:1": r.metadata!.name },
      segments: [{ text: "Build error log", spanId: "build:1" }],
    })
    let actual = combinedAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        msg: "Build error log",
        resourceName: "snack",
        level: FilterLevel.error,
        source: FilterSource.build,
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s UIResource: should show a crash rebuild alert  using the first build info", () => {
    let r: UIResource = k8sResource()
    r.status!.buildHistory = [
      {
        isCrashRebuild: true,
      },
      {
        isCrashRebuild: true,
      },
    ]
    let rInfo = r.status!.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podCreationTime = "10:00AM"

    let actual = combinedAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        msg: "Pod crashed",
        resourceName: "snack",
        level: FilterLevel.error,
        source: FilterSource.runtime,
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s UIResource: should show a warning alert using the first build history ", () => {
    let r: UIResource = k8sResource()
    r.status!.buildHistory = [
      {
        warnings: ["Hi i'm a warning"],
        finishTime: "10:00am",
        spanID: "build:2",
      },
      {
        warnings: ["This warning shouldn't show up", "Or this one"],
        spanID: "build:1",
      },
    ]

    appendLinesForManifestAndSpan(logStore, r.metadata!.name!, "build:1", [
      "build 1",
    ])
    appendLinesForManifestAndSpan(logStore, r.metadata!.name!, "build:2", [
      "build 2",
    ])

    let actual = combinedAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        msg: "Hi i'm a warning",
        resourceName: "snack",
        level: FilterLevel.warn,
        source: FilterSource.build,
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s UIResource: should show a pod restart alert and a build failed alert", () => {
    let r: UIResource = k8sResource()
    let rInfo = r.status!.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podRestarts = 1 // triggers pod restart alert
    rInfo.podCreationTime = "10:00AM"

    r.status!.buildHistory = [
      // triggers build failed alert
      {
        finishTime: "10:00AM",
        error: "build failed",
        spanID: "build:1",
      },
      {},
    ]
    logStore.append({
      spans: { "build:1": r.metadata!.name },
      segments: [{ text: "Build error log", spanId: "build:1" }],
    })

    let actual = combinedAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        msg: "Restarts: 1",
        resourceName: "snack",
        level: FilterLevel.warn,
        source: FilterSource.runtime,
      },
      {
        msg: "Build error log",
        resourceName: "snack",
        level: FilterLevel.error,
        source: FilterSource.build,
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s UIResource: should show 3 alerts: 1 crash rebuild alert, 1 build failed alert, 1 warning alert ", () => {
    let r: UIResource = k8sResource()
    r.status!.buildHistory = [
      {
        isCrashRebuild: true,
        warnings: ["Hi I am a warning"],
        error: "build failed",
        spanID: "build:1",
      },
    ]
    logStore.append({
      spans: { "build:1": r.metadata!.name },
      segments: [{ text: "Build failed log", spanId: "build:1" }],
    })
    let actual = combinedAlerts(r, logStore)
    let expectedAlerts: Alert[] = [
      {
        msg: "Pod crashed",
        resourceName: "snack",
        level: FilterLevel.error,
        source: FilterSource.runtime,
      },
      {
        msg: "Build failed log",
        resourceName: "snack",
        level: FilterLevel.error,
        source: FilterSource.build,
      },
      {
        msg: "Hi I am a warning",
        resourceName: "snack",
        level: FilterLevel.warn,
        source: FilterSource.build,
      },
    ]
    expect(actual).toEqual(expectedAlerts)
  })

  it("K8s UIResource: should show number of alerts a resource has", () => {
    let r: UIResource = k8sResource()
    let rInfo = r.status!.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podRestarts = 1
    let actualNum = combinedAlerts(r, null).length
    let expectedNum = 1

    expect(actualNum).toEqual(expectedNum)
  })
})

//DC UIResource Tests
it("DC UIResource: should show a warning alert using the first build history", () => {
  let r: UIResource = dcResource()
  r.status!.buildHistory = [
    {
      warnings: ["Hi i'm a warning"],
      finishTime: "10:00am",
      spanID: "build:2",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
      spanID: "build:1",
    },
  ]

  appendLinesForManifestAndSpan(logStore, r.metadata!.name!, "build:1", [
    "build 1",
  ])
  appendLinesForManifestAndSpan(logStore, r.metadata!.name!, "build:2", [
    "build 2",
  ])

  let actual = combinedAlerts(r, logStore)
  let expectedAlerts: Alert[] = [
    {
      msg: "Hi i'm a warning",
      resourceName: "vigoda",
      level: FilterLevel.warn,
      source: FilterSource.build,
    },
  ]
  expect(actual).toEqual(expectedAlerts)
})

it("DC UIResource has build failed alert using first build history info ", () => {
  let r: UIResource = dcResource()
  r.status!.buildHistory = [
    {
      spanID: "build:1",
      error: "theres an error !!!!",
      finishTime: "10:00am",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  logStore.append({
    spans: { "build:1": r.metadata!.name },
    segments: [{ text: "Hi you're build failed :'(", spanId: "build:1" }],
  })
  let actual = combinedAlerts(r, logStore)
  let expectedAlerts: Alert[] = [
    {
      msg: "Hi you're build failed :'(",
      resourceName: "vigoda",
      level: FilterLevel.error,
      source: FilterSource.build,
    },
  ]
  expect(actual).toEqual(expectedAlerts)
})

it("renders a build error for both a K8s resource and DC resource ", () => {
  let dcresource: UIResource = dcResource()
  dcresource.status!.buildHistory = [
    {
      error: "theres an error !!!!",
      finishTime: "10:00am",
      spanID: "build:1",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let k8sresource: UIResource = k8sResource()
  k8sresource.status!.buildHistory = [
    {
      error: "theres an error !!!!",
      finishTime: "10:00am",
      spanID: "build:2",
    },
    {
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  logStore.append({
    spans: {
      "build:1": { manifestName: dcresource.metadata!.name },
      "build:2": { manifestName: k8sresource.metadata!.name },
    },
    segments: [
      { text: "Hi your build failed :'(", spanId: "build:1" },
      { text: "Hi this (k8s) resource failed too :)", spanId: "build:2" },
    ],
  })

  let dcAlerts = combinedAlerts(dcresource, logStore)
  let k8sAlerts = combinedAlerts(k8sresource, logStore)

  let actual = dcAlerts.concat(k8sAlerts)
  let expectedAlerts: Alert[] = [
    {
      msg: "Hi your build failed :'(",
      resourceName: "vigoda",
      level: FilterLevel.error,
      source: FilterSource.build,
    },
    {
      msg: "Hi this (k8s) resource failed too :)",
      resourceName: "snack",
      level: FilterLevel.error,
      source: FilterSource.build,
    },
  ]
  expect(actual).toEqual(expectedAlerts)
})

function k8sResource(): UIResource {
  return {
    metadata: {
      name: "snack",
    },
    status: {
      buildHistory: [],
      endpointLinks: [],
      lastDeployTime: "",
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
      queued: false,
    },
  }
}

function dcResource(): UIResource {
  return {
    metadata: {
      name: "vigoda",
    },
    status: {
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
      },
      pendingBuildSince: "0001-01-01T00:00:00Z",
      hasPendingChanges: false,
      endpointLinks: [{ url: "http://localhost:9007/" }],
      runtimeStatus: "ok",
      queued: false,
    },
  }
}
