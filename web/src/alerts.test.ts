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
import { Resource, ResourceInfo, TriggerMode } from "./types"
import { emptyTypeAnnotation } from "@babel/types"

describe("getResourceAlerts", () => {
  it("shows that a pod status of error is an alert", () => {
    let r: Resource = emptyResource()
    r.ResourceInfo.PodStatus = "Error"
    r.ResourceInfo.PodStatusMessage = "I'm a pod in Error"

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

  it("should show pod restart alert", () => {
    let r: Resource = emptyResource()
    r.ResourceInfo.PodRestarts = 1

    let actual = getResourceAlerts(r)
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

  it("should show the first build alert", () => {
    let r: Resource = emptyResource()
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

  it("should show a crash rebuild alert  using the first build info", () => {
    let r: Resource = emptyResource()
    r.CrashLog = "Hello I am a crash log"
    r.ResourceInfo.PodCreationTime = "10:00AM"
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

  it("should show a warning alert using the first build history", () => {
    let r: Resource = emptyResource()
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

  it("should show a pod restart alert and a build failed alert", () => {
    let r: Resource = emptyResource()
    r.ResourceInfo.PodRestarts = 1 // triggers pod restart alert
    r.CrashLog = "I'm a pod that crashed"
    r.ResourceInfo.PodCreationTime = "10:00AM"
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
    let actual = getResourceAlerts(r)
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

  it("should show 3 alerts: 1 crash rebuild alert, 1 build failed alert, 1 warning alert ", () => {
    let r: Resource = emptyResource()
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

  it("should show number of alerts a resource has", () => {
    let r: Resource = emptyResource()
    r.ResourceInfo.PodRestarts = 1
    r.Alerts = getResourceAlerts(r)
    let actualNum = numberOfAlerts(r)
    let expectedNum = 1

    expect(actualNum).toEqual(expectedNum)
  })
})

function emptyResource(): Resource {
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
    ResourceInfo: {
      PodName: "testPod",
      PodCreationTime: "",
      PodUpdateStartTime: "",
      PodStatus: "",
      PodStatusMessage: "",
      PodRestarts: 0,
      PodLog: "",
      YAML: "",
      Endpoints: [],
    },
    RuntimeStatus: "",
    TriggerMode: TriggerMode.TriggerModeAuto,
    HasPendingChanges: true,
    Alerts: [],
  }
}
