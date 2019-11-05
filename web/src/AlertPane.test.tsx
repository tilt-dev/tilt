import React from "react"
import AlertPane from "./AlertPane"
import renderer from "react-test-renderer"
import { oneResourceUnrecognizedError } from "./testdata"
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
  resource.crashLog = "Eeeeek there is a problem"
  resource.buildHistory = [
    {
      log: "laa dee daa I'm not an error\nI'm serious",
      finishTime: ts,
      error: null,
    },
  ]
  if (!resource.k8sResourceInfo) throw new Error("Missing k8s info")
  resource.k8sResourceInfo.podCreationTime = ts
  resource.k8sResourceInfo.podStatus = "Error"
  resource.k8sResourceInfo.podRestarts = 2
  resource.alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(
      <AlertPane pathBuilder={pb} resources={resources as Array<Resource>} />
    )
    .toJSON()
  expect(tree).toMatchSnapshot()

  // the podStatus will flap between "Error" and "CrashLoopBackOff"
  resource.k8sResourceInfo.podStatus = "CrashLoopBackOff"
  resource.k8sResourceInfo.podRestarts = 3
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
  resource.crashLog = "Eeeeek there is a problem"
  let rInfo = resource.k8sResourceInfo
  if (!rInfo) throw new Error("Missing k8s info")
  rInfo.podName = "pod-name"
  rInfo.podCreationTime = ts
  rInfo.podStatus = "Running"
  rInfo.podRestarts = 2
  resource.alerts = getResourceAlerts(resource)

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
  resource.crashLog = "Eeeeek the container crashed"
  resource.buildHistory = [
    {
      log: "laa dee daa I'm not an error\nseriously",
      finishTime: ts,
      error: null,
    },
  ]
  if (!resource.k8sResourceInfo) throw new Error("missing k8s info")
  resource.k8sResourceInfo.podStatus = "ok"
  resource.k8sResourceInfo.podCreationTime = ts
  resource.k8sResourceInfo.podRestarts = 1
  resource.alerts = getResourceAlerts(resource)
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
  resource.crashLog = "Eeeeek the container crashed"
  resource.buildHistory = [
    {
      log: "laa dee daa I'm not an error\nseriously",
      finishTime: ts,
      error: null,
      isCrashRebuild: true,
    },
  ]
  if (!resource.k8sResourceInfo) throw new Error("missing k8s info")
  resource.k8sResourceInfo.podCreationTime = ts
  resource.k8sResourceInfo.podStatus = "ok"
  resource.alerts = getResourceAlerts(resource)

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
  resource.crashLog = "Eeeeek the container crashed\nno but really it crashed"
  resource.buildHistory = [
    {
      log: "laa dee daa I'm not an error\nseriously",
      finishTime: ts,
      error: null,
      isCrashRebuild: true,
    },
  ]
  if (!resource.k8sResourceInfo) throw new Error("missing k8s info")
  resource.k8sResourceInfo.podCreationTime = ts
  resource.k8sResourceInfo.podStatus = "ok"
  resource.alerts = getResourceAlerts(resource)

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
  resource.crashLog = "Eeeeek the container crashed"
  resource.buildHistory = [
    {
      log: "laa dee daa I'm not an error\nseriously",
      finishTime: ts,
      error: null,
      isCrashRebuild: true,
      warnings: ["Hi I'm a warning"],
    },
  ]
  if (!resource.k8sResourceInfo) throw new Error("missing k8s info")
  resource.k8sResourceInfo.podCreationTime = ts
  resource.k8sResourceInfo.podStatus = "ok"
  resource.alerts = getResourceAlerts(resource)

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
  resource.alerts = getResourceAlerts(resource)

  let resources = [resource]

  const tree = renderer
    .create(<AlertPane pathBuilder={pb} resources={resources} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})

//TODO TFT: Create tests testing that button appears and URL appears
function fillResourceFields(): Resource {
  return {
    name: "foo",
    combinedLog: "",
    buildHistory: [],
    crashLog: "",
    currentBuild: 0,
    directoriesWatched: [],
    endpoints: [],
    podID: "",
    isTiltfile: false,
    lastDeployTime: "",
    pathsWatched: [],
    pendingBuildEdits: [],
    pendingBuildReason: 0,
    pendingBuildSince: "",
    k8sResourceInfo: {
      podName: "",
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
    facets: [],
  }
}
