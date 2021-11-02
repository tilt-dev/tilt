import { mount } from "enzyme"
import fetchMock from "fetch-mock"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import LogStore from "./LogStore"
import PathBuilder from "./PathBuilder"
import { DEFAULT_OPTIONS } from "./ResourceListOptionsContext"
import SidebarItem from "./SidebarItem"
import { triggerUpdate } from "./SidebarItemView"
import SidebarResources from "./SidebarResources"
import SidebarTriggerButton, {
  SidebarTriggerButtonRoot,
  TriggerButtonTooltip,
} from "./SidebarTriggerButton"
import { oneResource, twoResourceView } from "./testdata"
import { ResourceName, ResourceView, TriggerMode } from "./types"

type UIResource = Proto.v1alpha1UIResource

let pathBuilder = PathBuilder.forTesting("localhost", "/")

let expectClickable = (button: any, expected: boolean) => {
  expect(button.hasClass("clickable")).toEqual(expected)
  expect(button.prop("disabled")).toEqual(!expected)
}
let expectManualTriggerIcon = (button: any, expected: boolean) => {
  let icon = expected ? "trigger-button-manual.svg" : "trigger-button.svg"
  expect(button.getDOMNode().innerHTML).toContain(icon)
}
let expectIsSelected = (button: any, expected: boolean) => {
  expect(button.hasClass("isSelected")).toEqual(expected)
}
let expectIsQueued = (button: any, expected: boolean) => {
  expect(button.hasClass("isQueued")).toEqual(expected)
}
let expectWithTooltip = (button: any, expected: string) => {
  expect(button.prop("title")).toEqual(expected)
}

let newSidebarItem = (r: UIResource): SidebarItem => {
  return new SidebarItem(r, new LogStore())
}

describe("SidebarTriggerButton", () => {
  beforeEach(() => {
    fetchMock.reset()
    mockAnalyticsCalls()
    fetchMock.mock("http://localhost/api/trigger", JSON.stringify({}))
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  it("POSTs to endpoint when clicked", () => {
    const root = mount(
      <SidebarTriggerButton
        isTiltfile={false}
        isSelected={true}
        triggerMode={TriggerMode.TriggerModeManualWithAutoInit}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
        onTrigger={() => triggerUpdate("doggos")}
      />
    )

    let element = root.find(SidebarTriggerButtonRoot)
    expect(element).toHaveLength(1)

    let preventDefaulted = false
    element.simulate("click", {
      preventDefault: () => {
        preventDefaulted = true
      },
    })
    expect(preventDefaulted).toEqual(true)

    expect(fetchMock.calls().length).toEqual(2)
    expectIncrs({
      name: "ui.web.triggerResource",
      tags: { action: AnalyticsAction.Click },
    })

    expect(fetchMock.calls()[1][0]).toEqual("http://localhost/api/trigger")
    expect(fetchMock.calls()[1][1]?.method).toEqual("post")
    expect(fetchMock.calls()[1][1]?.body).toEqual(
      JSON.stringify({
        manifest_names: ["doggos"],
        build_reason: 16 /* BuildReasonFlagTriggerWeb */,
      })
    )
  })

  it("disables button when resource is queued", () => {
    const root = mount(
      <SidebarTriggerButton
        isTiltfile={false}
        isSelected={true}
        triggerMode={TriggerMode.TriggerModeManualWithAutoInit}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={true}
        onTrigger={() => triggerUpdate("doggos")}
      />
    )

    let element = root.find(SidebarTriggerButtonRoot)
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.calls().length).toEqual(0)
  })

  it("shows the button for TriggerModeManual", () => {
    const root = mount(
      <SidebarTriggerButton
        isSelected={true}
        isTiltfile={false}
        triggerMode={TriggerMode.TriggerModeManual}
        hasBuilt={false}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
        onTrigger={() => triggerUpdate("doggos")}
      />
    )

    let element = root.find(SidebarTriggerButtonRoot)
    expectManualTriggerIcon(element, true)

    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.calls().length).toEqual(2)
  })

  it("shows clickable + clickMe trigger button for manual resource with pending changes", () => {
    let items = twoResourceView().uiResources.map(
      (r: UIResource, i: number) => {
        let res = r.status!
        res.triggerMode = TriggerMode.TriggerModeManualWithAutoInit // both manual
        res.currentBuild = {} // not currently building
        if (i == 0) {
          // only first resource has pending changes -- only this one should have class `isDirty`
          res.hasPendingChanges = true
          res.pendingBuildSince = new Date(Date.now()).toISOString()
        } else {
          res.hasPendingChanges = false
          res.pendingBuildSince = "0001-01-01T00:00:00Z"
        }

        return newSidebarItem(r)
      }
    )

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let buttons = root.find(SidebarTriggerButtonRoot)
    expect(buttons).toHaveLength(2)

    let b0 = buttons.at(0) // Manual resource with pending changes
    let b1 = buttons.at(1) // Manual resource, no pending changes

    expectClickable(b0, true)
    expectManualTriggerIcon(b0, true)
    expectIsQueued(b0, false)
    expectWithTooltip(b0, TriggerButtonTooltip.NeedsManualTrigger)

    expectClickable(b1, true)
    expectManualTriggerIcon(b1, false)
    expectIsQueued(b1, false)
    expectWithTooltip(b1, TriggerButtonTooltip.Default)
  })

  it("shows selected trigger button for selected resource", () => {
    let items = twoResourceView().uiResources.map(
      (r: UIResource, i: number) => {
        let res = r.status!
        res.triggerMode = TriggerMode.TriggerModeManualWithAutoInit // both manual
        res.currentBuild = {} // not currently building
        if (i == 0) {
          r.metadata = { name: "selected resource" }
        }

        return newSidebarItem(r)
      }
    )

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected="selected resource"
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let buttons = root.find(SidebarTriggerButtonRoot)
    expect(buttons).toHaveLength(2)

    expectIsSelected(buttons.at(0), true) // Selected resource
    expectIsSelected(buttons.at(1), false) // Non-selected resource
  })

  // A pending resource may mean that a pod is being rolled out, but is not
  // ready yet. In that case, the trigger button will delete the pod (cancelling
  // the rollout) and rebuild.
  it("shows clickMe trigger button when pending", () => {
    let items = twoResourceView().uiResources.map(
      (r: UIResource, i: number) => {
        let res = r.status!
        res.currentBuild = {} // not currently building

        if (i == 0) {
          res.hasPendingChanges = true
          res.pendingBuildSince = new Date(Date.now()).toISOString()
        } else {
          res.hasPendingChanges = false
          res.pendingBuildSince = "0001-01-01T00:00:00Z"
        }
        return newSidebarItem(r)
      }
    )

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let buttons = root.find(SidebarTriggerButtonRoot)
    expect(buttons).toHaveLength(2)
    let b0 = buttons.at(0) // Automatic resource with pending changes
    let b1 = buttons.at(1) // Automatic resource, no pending changes

    expectClickable(b0, true)
    expectManualTriggerIcon(b0, false)
    expectIsQueued(b0, false)
    expectWithTooltip(b0, TriggerButtonTooltip.Default)

    expectClickable(b1, true)
    expectManualTriggerIcon(b1, false)
    expectIsQueued(b1, false)
    expectWithTooltip(b1, TriggerButtonTooltip.Default)
  })

  it("trigger button not clickable if resource is building", () => {
    let res = oneResource() // by default this resource is in the process of building
    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(SidebarTriggerButtonRoot)
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectManualTriggerIcon(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, TriggerButtonTooltip.UpdateInProgOrPending)
  })

  it("trigger button not clickable if resource waiting for first build", () => {
    let r = oneResource()
    let res = r.status!
    res.currentBuild = {}
    res.buildHistory = []
    res.lastDeployTime = ""
    res.hasPendingChanges = false
    res.pendingBuildSince = ""
    let items = [newSidebarItem(r)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(SidebarTriggerButtonRoot)
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectManualTriggerIcon(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, TriggerButtonTooltip.UpdateInProgOrPending)
  })

  it("renders queued resource with class .isQueued and NOT .clickable", () => {
    let res = oneResource()
    res.status!.currentBuild = {}
    res.status!.queued = true
    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(SidebarTriggerButtonRoot)
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectManualTriggerIcon(button, false)
    expectIsQueued(button, true)
    expectWithTooltip(button, TriggerButtonTooltip.AlreadyQueued)
  })

  it("shows a trigger button for resource that failed its initial build", () => {
    let res = oneResource()
    res.status!.lastDeployTime = ""
    res.status!.currentBuild = {}
    res.status!.hasPendingChanges = false
    res.status!.pendingBuildSince = ""
    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(SidebarTriggerButtonRoot)
    expect(button).toHaveLength(1)

    expectClickable(button, true)
    expectManualTriggerIcon(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, TriggerButtonTooltip.Default)
  })

  it("shows trigger button for Tiltfile", () => {
    let res = oneResource()
    res.metadata = { name: ResourceName.tiltfile }
    res.status = res.status || {}
    res.status.currentBuild = {} // not currently building
    res.status.hasPendingChanges = false
    res.status.pendingBuildSince = "0001-01-01T00:00:00Z"

    let items = [newSidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          resourceListOptions={DEFAULT_OPTIONS}
        />
      </MemoryRouter>
    )

    let button = root.find(SidebarTriggerButtonRoot)
    expect(button).toHaveLength(1)

    expectClickable(button, true)
    expectManualTriggerIcon(button, false)
    expectIsQueued(button, false)
  })
})
