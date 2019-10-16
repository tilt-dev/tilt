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
    let rInfo = <K8sResourceInfo>r.K8sResourceInfo
    rInfo.PodStatus = "Error"
    rInfo.PodStatusMessage = "I'm a pod in Error"

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
    let rInfo = r.K8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.PodRestarts = 1
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
    r.BuildHistory = [
      {
        Log: "Build error log",
        FinishTime: "10:00AM",
      },
      {
        Log: "Build error 2",
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
    r.CrashLog = "Hello I am a crash log"
    r.BuildHistory = [
      {
        Log: "Hello I am a log",
        IsCrashRebuild: true,
        Error: null,
      },
      {
        Log: "Hello I am a log 2 ",
        IsCrashRebuild: true,
        Error: null,
      },
    ]
    let rInfo = r.K8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.PodCreationTime = "10:00AM"

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
    r.BuildHistory = [
      {
        Log: "Hello I'm a log",
        Warnings: ["Hi i'm a warning"],
        Error: null,
        FinishTime: "10:00am",
      },
      {
        Log: "Hello I'm a log2",
        Warnings: ["This warning shouldn't show up", "Or this one"],
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
    let rInfo = r.K8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.PodRestarts = 1 // triggers pod restart alert
    rInfo.PodCreationTime = "10:00AM"

    r.CrashLog = "I'm a pod that crashed"
    r.BuildHistory = [
      // triggers build failed alert
      {
        Log: "Build error log",
        FinishTime: "10:00AM",
      },
      {
        Log: "Build error 2",
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
    r.CrashLog = "Hello I am a crash log"
    r.BuildHistory = [
      {
        Log: "Build failed log",
        IsCrashRebuild: true,
        Warnings: ["Hi I am a warning"],
        FinishTime: "10:00am",
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
    let rInfo = r.K8sResourceInfo
    if (!rInfo) throw new Error("missing k8s info")
    rInfo.PodRestarts = 1
    r.Alerts = getResourceAlerts(r)
    let actualNum = numberOfAlerts(r)
    let expectedNum = 1

    expect(actualNum).toEqual(expectedNum)
  })
})

//DC Resource Tests
it("DC Resource: should show a warning alert using the first build history ", () => {
  let r: Resource = dcResource()
  r.BuildHistory = [
    {
      Log: "Hello I'm a log",
      Warnings: ["Hi i'm a warning"],
      Error: null,
      FinishTime: "10:00am",
    },
    {
      Log: "Hello I'm a log2",
      Warnings: ["This warning shouldn't show up", "Or this one"],
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
it("DC Resource: should show a warning alert using the first build history ", () => {
  let r: Resource = dcResource()
  r.BuildHistory = [
    {
      Log: "Hello I'm a log",
      Warnings: ["Hi i'm a warning"],
      Error: null,
      FinishTime: "10:00am",
    },
    {
      Log: "Hello I'm a log2",
      Warnings: ["This warning shouldn't show up", "Or this one"],
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
  r.BuildHistory = [
    {
      Log: "Hi you're build failed :'(",
      Error: "theres an error !!!!",
      FinishTime: "10:00am",
    },
    {
      Log: "Hello I'm a log2",
      Warnings: ["This warning shouldn't show up", "Or this one"],
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
  dcresource.BuildHistory = [
    {
      Log: "Hi your build failed :'(",
      Error: "theres an error !!!!",
      FinishTime: "10:00am",
    },
    {
      Log: "Hello I'm a log2",
      Warnings: ["This warning shouldn't show up", "Or this one"],
    },
  ]
  let k8sresource: Resource = k8sResource()
  k8sresource.BuildHistory = [
    {
      Log: "Hi this (k8s) resource failed too :)",
      Error: "theres an error !!!!",
      FinishTime: "10:00am",
    },
    {
      Log: "Hello I'm a log2",
      Warnings: ["This warning shouldn't show up", "Or this one"],
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
    Name: "snack",
    CombinedLog: "",
    BuildHistory: [],
    CrashLog: "",
    CurrentBuild: 0,
    DirectoriesWatched: [],
    Endpoints: [],
    PodID: "podID",
    IsTiltfile: false,
    LastDeployTime: "",
    PathsWatched: [],
    PendingBuildEdits: [],
    PendingBuildReason: 0,
    PendingBuildSince: "",
    K8sResourceInfo: {
      PodName: "testPod",
      PodCreationTime: "",
      PodUpdateStartTime: "",
      PodStatus: "",
      PodStatusMessage: "",
      PodRestarts: 0,
      PodLog: "",
      Endpoints: [],
    },
    RuntimeStatus: "",
    TriggerMode: TriggerMode.TriggerModeAuto,
    HasPendingChanges: true,
    Alerts: [],
  }
}

function dcResource(): Resource {
  return {
    Name: "vigoda",
    DirectoriesWatched: [],
    PathsWatched: ["Tiltfile.dc"],
    LastDeployTime: "2019-08-07T11:43:37.568629-04:00",
    TriggerMode: 0,
    BuildHistory: [
      {
        Edits: null,
        Error: null,
        Warnings: null,
        StartTime: "2019-08-07T11:43:32.422237-04:00",
        FinishTime: "2019-08-07T11:43:37.568626-04:00",
        Log: "",
        IsCrashRebuild: false,
      },
    ],
    CurrentBuild: {
      Edits: null,
      Error: null,
      Warnings: null,
      StartTime: "0001-01-01T00:00:00Z",
      FinishTime: "0001-01-01T00:00:00Z",
      Log: "",
      IsCrashRebuild: false,
    },
    PendingBuildReason: 0,
    PendingBuildEdits: [],
    PendingBuildSince: "0001-01-01T00:00:00Z",
    HasPendingChanges: false,
    Endpoints: ["http://localhost:9007/"],
    PodID: "",
    DCResourceInfo: {
      ConfigPaths: [""],
      ContainerStatus: "OK",
      ContainerID: "",
      Log: "",
      StartTime: "2019-08-07T11:43:36.900841-04:00",
    },
    RuntimeStatus: "ok",
    IsTiltfile: false,
    CombinedLog: "",
    CrashLog: "",
    Alerts: [],
  }
}
