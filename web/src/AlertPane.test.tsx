import React from "react"
import AlertPane from "./AlertPane"
import renderer from "react-test-renderer"
import { oneResourceUnrecognizedError } from "./testdata.test"
import { Resource, TriggerMode } from "./types"
import { getResourceAlerts } from "./alerts"
import PathBuilder from "./PathBuilder"
import { mount } from "enzyme"

let pb = new PathBuilder("localhost", "")
beforeEach(() => {
  fetchMock.resetMocks()
  Date.now = jest.fn(() => 1482363367071)
})

it("renders no errors", () => {
  let resource = fillResourceFields()
  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
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
  if (!resource.K8sResourceInfo) throw new Error("Missing k8s info")
  resource.K8sResourceInfo.PodCreationTime = ts
  resource.K8sResourceInfo.PodStatus = "Error"
  resource.K8sResourceInfo.PodRestarts = 2
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()

  // the podStatus will flap between "Error" and "CrashLoopBackOff"
  resource.K8sResourceInfo.PodStatus = "CrashLoopBackOff"
  resource.K8sResourceInfo.PodRestarts = 3
  const newTree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
    )
    .toJSON()
  expect(newTree).toMatchSnapshot()
})

it("renders pod restart dismiss button", () => {
  let resource = fillResourceFields()
  const ts = "1,555,970,585,039"
  resource.CrashLog = "Eeeeek there is a problem"
  let rInfo = resource.K8sResourceInfo
  if (!rInfo) throw new Error("Missing k8s info")
  rInfo.PodName = "pod-name"
  rInfo.PodCreationTime = ts
  rInfo.PodStatus = "Running"
  rInfo.PodRestarts = 2
  resource.Alerts = getResourceAlerts(resource)

  let resources: Array<Resource> = [resource]

  let root = mount(<AlertPane pathBuilder={pb} resources={resources} />)
  let button = root.find(".AlertPane-dismissButton")
  expect(button).toHaveLength(1)
  fetchMock.mockResponse(JSON.stringify({}))
  button.simulate("click")

  expect(fetchMock.mock.calls.length).toEqual(1)
  expect(fetchMock.mock.calls[0][0]).toEqual("/api/action")
  expect(fetchMock.mock.calls[0][1].body).toEqual(
    JSON.stringify({
      type: "PodResetRestarts",
      manifest_name: "foo",
      visible_restarts: 2,
      pod_id: "pod-name",
    })
  )
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
  if (!resource.K8sResourceInfo) throw new Error("missing k8s info")
  resource.K8sResourceInfo.PodStatus = "ok"
  resource.K8sResourceInfo.PodCreationTime = ts
  resource.K8sResourceInfo.PodRestarts = 1
  resource.Alerts = getResourceAlerts(resource)
  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
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
  if (!resource.K8sResourceInfo) throw new Error("missing k8s info")
  resource.K8sResourceInfo.PodCreationTime = ts
  resource.K8sResourceInfo.PodStatus = "ok"
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
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
  if (!resource.K8sResourceInfo) throw new Error("missing k8s info")
  resource.K8sResourceInfo.PodCreationTime = ts
  resource.K8sResourceInfo.PodStatus = "ok"
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
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
  if (!resource.K8sResourceInfo) throw new Error("missing k8s info")
  resource.K8sResourceInfo.PodCreationTime = ts
  resource.K8sResourceInfo.PodStatus = "ok"
  resource.Alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
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
    .create(<AlertPane pathBuilder={pb} resources={resources} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})

//TODO TFT: Create tests testing that button appears and URL appears
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
    K8sResourceInfo: {
      PodName: "",
      PodCreationTime: "",
      PodUpdateStartTime: "",
      PodStatus: "",
      PodStatusMessage: "",
      PodRestarts: 0,
      PodLog: "",
    },
    RuntimeStatus: "",
    TriggerMode: TriggerMode.TriggerModeAuto,
    HasPendingChanges: true,
    Alerts: [],
  }
}
