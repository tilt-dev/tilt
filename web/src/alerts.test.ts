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
import { Resource, K8sResourceInfo, TriggerMode } from "./types"

describe("getResourceAlerts", () => {
  it("K8Resource: shows that a pod status of error is an alert", () => {
    let r: Resource = k8sResource()
    let rInfo = <K8sResourceInfo>r.k8sResourceInfo
    rInfo.podStatus = "Error"
    rInfo.podStatusMessage = "I'm a pod in Error"

    let actual = getResourceAlerts(r)
    let expectedAlerts: Array<Alert> = [
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
    let actual = getResourceAlerts(r).map(a => {
      delete a.dismissHandler
      return a
    })
    let expectedAlerts: Array<Alert> = [
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
        log: "Build error log",
        finishTime: "10:00AM",
        error: "build failed",
      },
      {
        log: "Build error 2",
      },
    ]
    let actual = getResourceAlerts(r)
    let expectedAlerts: Array<Alert> = [
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
        log: "Hello I am a log",
        isCrashRebuild: true,
        error: null,
      },
      {
        log: "Hello I am a log 2 ",
        isCrashRebuild: true,
        error: null,
      },
    ]
    let rInfo = r.k8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.podCreationTime = "10:00AM"

    let actual = getResourceAlerts(r)
    let expectedAlerts: Array<Alert> = [
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
        log: "Hello I'm a log",
        warnings: ["Hi i'm a warning"],
        error: null,
        finishTime: "10:00am",
      },
      {
        log: "Hello I'm a log2",
        warnings: ["This warning shouldn't show up", "Or this one"],
      },
    ]
    let actual = getResourceAlerts(r)
    let expectedAlerts: Array<Alert> = [
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
        log: "Build error log",
        finishTime: "10:00AM",
        error: "build failed",
      },
      {
        log: "Build error 2",
      },
    ]
    let actual = getResourceAlerts(r).map(a => {
      delete a.dismissHandler
      return a
    })
    let expectedAlerts: Array<Alert> = [
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
        log: "Build failed log",
        isCrashRebuild: true,
        warnings: ["Hi I am a warning"],
        finishTime: "10:00am",
        error: "build failed",
      },
    ]
    let actual = getResourceAlerts(r)
    let expectedAlerts: Array<Alert> = [
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
    r.alerts = getResourceAlerts(r)
    let actualNum = numberOfAlerts(r)
    let expectedNum = 1

    expect(actualNum).toEqual(expectedNum)
  })
})

//DC Resource Tests
it("DC Resource: should show a warning alert using the first build history ", () => {
  let r: Resource = dcResource()
  r.buildHistory = [
    {
      log: "Hello I'm a log",
      warnings: ["Hi i'm a warning"],
      error: null,
      finishTime: "10:00am",
    },
    {
      log: "Hello I'm a log2",
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let actual = getResourceAlerts(r)
  let expectedAlerts: Array<Alert> = [
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
it("DC resource: should show a warning alert using the first build history ", () => {
  let r: Resource = dcResource()
  r.buildHistory = [
    {
      log: "Hello I'm a log",
      warnings: ["Hi i'm a warning"],
      error: null,
      finishTime: "10:00am",
    },
    {
      log: "Hello I'm a log2",
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let actual = getResourceAlerts(r)
  let expectedAlerts: Array<Alert> = [
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
      log: "Hi you're build failed :'(",
      error: "theres an error !!!!",
      finishTime: "10:00am",
    },
    {
      log: "Hello I'm a log2",
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let actual = getResourceAlerts(r)
  let expectedAlerts: Array<Alert> = [
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
      log: "Hi your build failed :'(",
      error: "theres an error !!!!",
      finishTime: "10:00am",
    },
    {
      log: "Hello I'm a log2",
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let k8sresource: Resource = k8sResource()
  k8sresource.buildHistory = [
    {
      log: "Hi this (k8s) resource failed too :)",
      error: "theres an error !!!!",
      finishTime: "10:00am",
    },
    {
      log: "Hello I'm a log2",
      warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]

  let dcAlerts = getResourceAlerts(dcresource)
  let k8sAlerts = getResourceAlerts(k8sresource)

  let actual = dcAlerts.concat(k8sAlerts)
  let expectedAlerts: Array<Alert> = [
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
    combinedLog: "",
    buildHistory: [],
    crashLog: "",
    currentBuild: 0,
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
      podLog: "",
    },
    runtimeStatus: "",
    triggerMode: TriggerMode.TriggerModeAuto,
    hasPendingChanges: true,
    alerts: [],
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
        edits: null,
        error: null,
        warnings: null,
        startTime: "2019-08-07T11:43:32.422237-04:00",
        finishTime: "2019-08-07T11:43:37.568626-04:00",
        log: "",
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
    pendingBuildEdits: [],
    pendingBuildSince: "0001-01-01T00:00:00Z",
    hasPendingChanges: false,
    endpoints: ["http://localhost:9007/"],
    podID: "",
    dcResourceInfo: {
      configPaths: [""],
      containerStatus: "OK",
      containerID: "",
      log: "",
      startTime: "2019-08-07T11:43:36.900841-04:00",
    },
    runtimeStatus: "ok",
    isTiltfile: false,
    combinedLog: "",
    crashLog: "",
    alerts: [],
  }
}
