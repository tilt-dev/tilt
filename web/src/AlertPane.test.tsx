import React from "react"
import AlertPane from "./AlertPane"
import renderer from "react-test-renderer"
import { oneResourceUnrecognizedError } from "./testdata.test"
import { Resource, ResourceInfo, TriggerMode } from "./types"
import {
  Alert,
  PodRestartErrorType,
  PodStatusErrorType,
  CrashRebuildErrorType,
  BuildFailedErrorType,
  WarningErrorType,
  getResourceAlerts,
} from "./alerts"

const fakeSendAlert = () => {}

beforeEach(() => {
  Date.now = jest.fn(() => 1482363367071)
})

it("renders no errors", () => {
  let resource = fillResourceFields()
  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders one container start error", () => {
  const ts = "1,555,970,585,039"

  let resource = fillResourceFields()
  resource.CrashLog = "Eeeeek there is a problem"
  resource.BuildHistory = [
    {
      Log: "laa dee daa I'm not an error\nI'm serious",
      FinishTime: ts,
      Error: null,
    },
  ]
  resource.ResourceInfo.PodCreationTime = ts
  resource.ResourceInfo.PodStatus = "Error"
  resource.ResourceInfo.PodRestarts = 2
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()

  // the podStatus will flap between "Error" and "CrashLoopBackOff"
  resource.ResourceInfo.PodStatus = "CrashLoopBackOff"
  resource.ResourceInfo.PodRestarts = 3

  const newTree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()
  expect(newTree).toMatchSnapshot()
})

it("shows that a container has restarted", () => {
  const ts = "1,555,970,585,039"
  let resource = fillResourceFields()
  resource.CrashLog = "Eeeeek the container crashed"
  resource.BuildHistory = [
    {
      Log: "laa dee daa I'm not an error\nseriously",
      FinishTime: ts,
      Error: null,
    },
  ]
  resource.ResourceInfo.PodStatus = "ok"
  resource.ResourceInfo.PodCreationTime = ts
  resource.ResourceInfo.PodRestarts = 1
  resource.Alerts = getResourceAlerts(resource)
  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("shows that a crash rebuild has occurred", () => {
  const ts = "1,555,970,585,039"
  let resource = fillResourceFields()
  resource.CrashLog = "Eeeeek the container crashed"
  resource.BuildHistory = [
    {
      Log: "laa dee daa I'm not an error\nseriously",
      FinishTime: ts,
      Error: null,
      IsCrashRebuild: true,
    },
  ]
  resource.ResourceInfo.PodCreationTime = ts
  resource.ResourceInfo.PodStatus = "ok"
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("renders multiple lines of a crash log", () => {
  const ts = "1,555,970,585,039"

  let resource = fillResourceFields()
  resource.CrashLog = "Eeeeek the container crashed\nno but really it crashed"
  resource.BuildHistory = [
    {
      Log: "laa dee daa I'm not an error\nseriously",
      FinishTime: ts,
      Error: null,
      IsCrashRebuild: true,
    },
  ]
  resource.ResourceInfo.PodCreationTime = ts
  resource.ResourceInfo.PodStatus = "ok"
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("renders warnings", () => {
  const ts = "1,555,970,585,039"
  let resource = fillResourceFields()
  resource.CrashLog = "Eeeeek the container crashed"
  resource.BuildHistory = [
    {
      Log: "laa dee daa I'm not an error\nseriously",
      FinishTime: ts,
      Error: null,
      IsCrashRebuild: true,
      Warnings: ["Hi I'm a warning"],
    },
  ]
  resource.ResourceInfo.PodCreationTime = ts
  resource.ResourceInfo.PodStatus = "ok"
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("renders one container unrecognized error", () => {
  const ts = "1,555,970,585,039"
  let resource = oneResourceUnrecognizedError()
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources}
        teamAlertsIsEnabled={false}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("renders the get alert link button when the feature is enabled", () => {
  const ts = "1,555,970,585,039"
  let resources: Array<Partial<Resource>> = [
    {
      Name: "foo",
      ResourceInfo: {
        PodName: "",
        PodCreationTime: "",
        PodUpdateStartTime: "",
        PodStatus: "",
        PodStatusMessage: "",
        PodRestarts: 0,
        PodLog: "",
        YAML: "",
        Endpoints: [],
      },
      BuildHistory: [],
      Alerts: [
        {
          alertType: BuildFailedErrorType,
          msg: "laa dee daa I'm an error\nfor real I am",
          header: "",
          timestamp: ts,
          resourceName: "foo",
        },
      ],
    },
  ]

  const tree = renderer
    .create(
      <AlertPane
        resources={resources as Array<Resource>}
        teamAlertsIsEnabled={true}
        handleSendAlert={fakeSendAlert}
        alertLinks={{ BuildErrorFoo: "https://alerts.tilt.dev/alert/moo" }}
      />
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

function fillResourceFields(): Resource {
  return {
    Name: "foo",
    CombinedLog: "",
    BuildHistory: [],
    CrashLog: "",
    CurrentBuild: 0,
    DirectoriesWatched: [],
    Endpoints: [],
    PodID: "",
    IsTiltfile: false,
    LastDeployTime: "",
    PathsWatched: [],
    PendingBuildEdits: [],
    PendingBuildReason: 0,
    PendingBuildSince: "",
    ResourceInfo: {
      PodName: "",
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
