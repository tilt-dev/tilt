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
    let r: Resource = emptyK8Resource()
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

  it("should show pod restart alert for K8s resource", () => {
    let r: Resource = emptyK8Resource()
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

  it("should show the first build alert for K8s resource", () => {
    let r: Resource = emptyK8Resource()
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
    let r: Resource = emptyK8Resource()
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

  it("should show a warning alert using the first build history for K8s Resource", () => {
    let r: Resource = emptyK8Resource()
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

  it("should show a pod restart alert and a build failed alert for K8s resource", () => {
    let r: Resource = emptyK8Resource()
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
    let r: Resource = emptyK8Resource()
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
    let r: Resource = emptyK8Resource()
    r.ResourceInfo.PodRestarts = 1
    r.Alerts = getResourceAlerts(r)
    let actualNum = numberOfAlerts(r)
    let expectedNum = 1

    expect(actualNum).toEqual(expectedNum)
  })
})

function emptyK8Resource(): Resource {
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
        Reason: 8,
        Log:
          "\n\u001b[34m──┤ Building: \u001b[0mvigoda\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mBuilding Dockerfile: [docker.io/library/vigoda_for_dc]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/vigoda\n  RUN go install github.com/windmilleng/servantes/vigoda\n  \n  ENV TEMPLATE_DIR /go/src/github.com/windmilleng/servantes/vigoda/web/templates\n  \n  ENTRYPOINT /go/bin/vigoda\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ copy /context /\n    ╎ copy /context / done | 224ms\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] done | 1.836s\n    ╎ exporting to image\n\nCreating servantes_vigoda_1 ... \r\n\u001b[1A\u001b[2K\rCreating servantes_vigoda_1 ... \u001b[32mdone\u001b[0m\r\u001b[1B\u001b[34m  │ \u001b[0mStep 1 - 2.735s\n\u001b[34m  │ \u001b[0mDone in: 5.145s \n\n",
        IsCrashRebuild: false,
      },
    ],
    CurrentBuild: {
      Edits: null,
      Error: null,
      Warnings: null,
      StartTime: "0001-01-01T00:00:00Z",
      FinishTime: "0001-01-01T00:00:00Z",
      Reason: 0,
      Log: "",
      IsCrashRebuild: false,
    },
    PendingBuildReason: 0,
    PendingBuildEdits: [],
    PendingBuildSince: "0001-01-01T00:00:00Z",
    HasPendingChanges: false,
    Endpoints: ["http://localhost:9007/"],
    PodID: "",
    ResourceInfo: {
      ConfigPaths: [
        "/Users/dan/go/src/github.com/windmilleng/servantes/docker-compose.yml",
      ],
      ContainerStatus: "OK",
      ContainerID:
        "8c7e307b781efbeb234f15c1d116135aaaff9b9b6f972d57d8e61d1cb307ee7f",
      Log:
        "\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:38.877497931Z 2019/08/07 15:43:38 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:40.876938430Z 2019/08/07 15:43:40 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:42.879584079Z 2019/08/07 15:43:42 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:44.878642925Z 2019/08/07 15:43:44 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:46.879662719Z 2019/08/07 15:43:46 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:48.882274621Z 2019/08/07 15:43:48 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:50.861075847Z 2019/08/07 15:43:50 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:52.861272944Z 2019/08/07 15:43:52 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:54.861524440Z 2019/08/07 15:43:54 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:56.861696610Z 2019/08/07 15:43:56 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:58.862126678Z 2019/08/07 15:43:58 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:00.862754783Z 2019/08/07 15:44:00 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:02.863098595Z 2019/08/07 15:44:02 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:04.863405587Z 2019/08/07 15:44:04 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:06.863710630Z 2019/08/07 15:44:06 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:08.864700524Z 2019/08/07 15:44:08 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:10.865846654Z 2019/08/07 15:44:10 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:12.866805855Z 2019/08/07 15:44:12 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:14.866990429Z 2019/08/07 15:44:14 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:16.868635304Z 2019/08/07 15:44:16 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:18.871037376Z 2019/08/07 15:44:18 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:20.850004669Z 2019/08/07 15:44:20 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:22.850199360Z 2019/08/07 15:44:22 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:24.850466551Z 2019/08/07 15:44:24 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:26.850814196Z 2019/08/07 15:44:26 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:28.851385278Z 2019/08/07 15:44:28 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:30.856848209Z 2019/08/07 15:44:30 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:32.852509949Z 2019/08/07 15:44:32 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:34.853034950Z 2019/08/07 15:44:34 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:36.853769552Z 2019/08/07 15:44:36 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:38.855119725Z 2019/08/07 15:44:38 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:40.855541030Z 2019/08/07 15:44:40 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:42.856937115Z 2019/08/07 15:44:42 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:44.857522514Z 2019/08/07 15:44:44 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:46.858652465Z 2019/08/07 15:44:46 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:48.859603605Z 2019/08/07 15:44:48 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:50.838684337Z 2019/08/07 15:44:50 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:52.839122921Z 2019/08/07 15:44:52 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:54.839763392Z 2019/08/07 15:44:54 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:56.840349509Z 2019/08/07 15:44:56 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:58.840822171Z 2019/08/07 15:44:58 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:00.840846852Z 2019/08/07 15:45:00 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:02.841764655Z 2019/08/07 15:45:02 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:04.842121588Z 2019/08/07 15:45:04 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:06.842686609Z 2019/08/07 15:45:06 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:08.843235140Z 2019/08/07 15:45:08 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:10.843617268Z 2019/08/07 15:45:10 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:12.844101030Z 2019/08/07 15:45:12 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:14.844335731Z 2019/08/07 15:45:14 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:16.845612367Z 2019/08/07 15:45:16 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:18.846818405Z 2019/08/07 15:45:18 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:20.825827247Z 2019/08/07 15:45:20 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:22.826246643Z 2019/08/07 15:45:22 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:24.826573026Z 2019/08/07 15:45:24 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:26.826906606Z 2019/08/07 15:45:26 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:28.827086696Z 2019/08/07 15:45:28 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:30.828016224Z 2019/08/07 15:45:30 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:32.830856549Z 2019/08/07 15:45:32 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:34.831068046Z 2019/08/07 15:45:34 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:36.831767838Z 2019/08/07 15:45:36 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:38.832336622Z 2019/08/07 15:45:38 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:40.833605370Z 2019/08/07 15:45:40 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:42.834266733Z 2019/08/07 15:45:42 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:44.835463644Z 2019/08/07 15:45:44 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:46.836028837Z 2019/08/07 15:45:46 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:48.836319511Z 2019/08/07 15:45:48 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:50.815110337Z 2019/08/07 15:45:50 Server status: All good\n",
      StartTime: "2019-08-07T11:43:36.900841-04:00",
    },
    RuntimeStatus: "ok",
    IsTiltfile: false,
    ShowBuildStatus: true,
    CombinedLog:
      "\n\u001b[34m──┤ Building: \u001b[0mvigoda\u001b[34m ├──────────────────────────────────────────────\u001b[0m\n\u001b[34mSTEP 1/1 — \u001b[0mBuilding Dockerfile: [docker.io/library/vigoda_for_dc]\nBuilding Dockerfile:\n  FROM golang:1.10\n  \n  ADD . /go/src/github.com/windmilleng/servantes/vigoda\n  RUN go install github.com/windmilleng/servantes/vigoda\n  \n  ENV TEMPLATE_DIR /go/src/github.com/windmilleng/servantes/vigoda/web/templates\n  \n  ENTRYPOINT /go/bin/vigoda\n\n\n\u001b[34m  │ \u001b[0mTarring context…\n\u001b[34m  │ \u001b[0mBuilding image\n    ╎ copy /context /\n    ╎ copy /context / done | 224ms\n    ╎ [1/3] FROM docker.io/library/golang:1.10\n    ╎ [2/3] ADD . /go/src/github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] RUN go install github.com/windmilleng/servantes/vigoda\n    ╎ [3/3] done | 1.836s\n    ╎ exporting to image\n\nCreating servantes_vigoda_1 ... \r\n\u001b[1A\u001b[2K\rCreating servantes_vigoda_1 ... \u001b[32mdone\u001b[0m\r\u001b[1B\u001b[34m  │ \u001b[0mStep 1 - 2.735s\n\u001b[34m  │ \u001b[0mDone in: 5.145s \n\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:38.877497931Z 2019/08/07 15:43:38 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:40.876938430Z 2019/08/07 15:43:40 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:42.879584079Z 2019/08/07 15:43:42 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:44.878642925Z 2019/08/07 15:43:44 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:46.879662719Z 2019/08/07 15:43:46 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:48.882274621Z 2019/08/07 15:43:48 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:50.861075847Z 2019/08/07 15:43:50 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:52.861272944Z 2019/08/07 15:43:52 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:54.861524440Z 2019/08/07 15:43:54 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:56.861696610Z 2019/08/07 15:43:56 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:43:58.862126678Z 2019/08/07 15:43:58 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:00.862754783Z 2019/08/07 15:44:00 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:02.863098595Z 2019/08/07 15:44:02 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:04.863405587Z 2019/08/07 15:44:04 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:06.863710630Z 2019/08/07 15:44:06 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:08.864700524Z 2019/08/07 15:44:08 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:10.865846654Z 2019/08/07 15:44:10 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:12.866805855Z 2019/08/07 15:44:12 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:14.866990429Z 2019/08/07 15:44:14 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:16.868635304Z 2019/08/07 15:44:16 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:18.871037376Z 2019/08/07 15:44:18 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:20.850004669Z 2019/08/07 15:44:20 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:22.850199360Z 2019/08/07 15:44:22 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:24.850466551Z 2019/08/07 15:44:24 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:26.850814196Z 2019/08/07 15:44:26 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:28.851385278Z 2019/08/07 15:44:28 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:30.856848209Z 2019/08/07 15:44:30 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:32.852509949Z 2019/08/07 15:44:32 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:34.853034950Z 2019/08/07 15:44:34 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:36.853769552Z 2019/08/07 15:44:36 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:38.855119725Z 2019/08/07 15:44:38 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:40.855541030Z 2019/08/07 15:44:40 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:42.856937115Z 2019/08/07 15:44:42 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:44.857522514Z 2019/08/07 15:44:44 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:46.858652465Z 2019/08/07 15:44:46 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:48.859603605Z 2019/08/07 15:44:48 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:50.838684337Z 2019/08/07 15:44:50 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:52.839122921Z 2019/08/07 15:44:52 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:54.839763392Z 2019/08/07 15:44:54 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:56.840349509Z 2019/08/07 15:44:56 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:44:58.840822171Z 2019/08/07 15:44:58 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:00.840846852Z 2019/08/07 15:45:00 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:02.841764655Z 2019/08/07 15:45:02 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:04.842121588Z 2019/08/07 15:45:04 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:06.842686609Z 2019/08/07 15:45:06 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:08.843235140Z 2019/08/07 15:45:08 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:10.843617268Z 2019/08/07 15:45:10 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:12.844101030Z 2019/08/07 15:45:12 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:14.844335731Z 2019/08/07 15:45:14 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:16.845612367Z 2019/08/07 15:45:16 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:18.846818405Z 2019/08/07 15:45:18 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:20.825827247Z 2019/08/07 15:45:20 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:22.826246643Z 2019/08/07 15:45:22 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:24.826573026Z 2019/08/07 15:45:24 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:26.826906606Z 2019/08/07 15:45:26 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:28.827086696Z 2019/08/07 15:45:28 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:30.828016224Z 2019/08/07 15:45:30 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:32.830856549Z 2019/08/07 15:45:32 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:34.831068046Z 2019/08/07 15:45:34 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:36.831767838Z 2019/08/07 15:45:36 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:38.832336622Z 2019/08/07 15:45:38 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:40.833605370Z 2019/08/07 15:45:40 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:42.834266733Z 2019/08/07 15:45:42 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:44.835463644Z 2019/08/07 15:45:44 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:46.836028837Z 2019/08/07 15:45:46 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:48.836319511Z 2019/08/07 15:45:48 Server status: All good\n\u001b[36mvigoda_1       |\u001b[0m 2019-08-07T15:45:50.815110337Z 2019/08/07 15:45:50 Server status: All good\n",
    CrashLog: "",
    Alerts: [],
  }
}
