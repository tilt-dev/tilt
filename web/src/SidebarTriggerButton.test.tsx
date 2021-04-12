import { mount } from "enzyme"
import fetchMock from "fetch-mock"
import React from "react"
import { MemoryRouter } from "react-router"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import { triggerUpdate } from "./SidebarItemView"
import SidebarResources from "./SidebarResources"
import SidebarTriggerButton, {
  SidebarTriggerButtonRoot,
  TriggerButtonTooltip,
} from "./SidebarTriggerButton"
import { oneResource, twoResourceView } from "./testdata"
import { ResourceName, ResourceView, TriggerMode } from "./types"

type Resource = Proto.webviewResource

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
    expectIncrs({ name: "ui.web.triggerResource", tags: { action: "click" } })

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
    let items = twoResourceView().resources.map((res: Resource, i: number) => {
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

      return new SidebarItem(res)
    })

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
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
    expectWithTooltip(b1, TriggerButtonTooltip.ClickToForce)
  })

  it("shows selected trigger button for selected resource", () => {
    let items = twoResourceView().resources.map((res: Resource, i: number) => {
      res.triggerMode = TriggerMode.TriggerModeManualWithAutoInit // both manual
      res.currentBuild = {} // not currently building
      if (i == 0) {
        res.name = "selected resource"
      }

      return new SidebarItem(res)
    })

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected="selected resource"
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let buttons = root.find(SidebarTriggerButtonRoot)
    expect(buttons).toHaveLength(2)

    expectIsSelected(buttons.at(0), true) // Selected resource
    expectIsSelected(buttons.at(1), false) // Non-selected resource
  })

  it("never shows clickMe trigger button for automatic resources", () => {
    let items = twoResourceView().resources.map((res: Resource, i: number) => {
      res.currentBuild = {} // not currently building

      if (i == 0) {
        // first resource has pending changes -- but is automatic, should NOT
        // have a clickMe button (and button should be !clickable)
        res.hasPendingChanges = true
        res.pendingBuildSince = new Date(Date.now()).toISOString()
      } else {
        res.hasPendingChanges = false
        res.pendingBuildSince = "0001-01-01T00:00:00Z"
      }
      return new SidebarItem(res)
    })

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let buttons = root.find(SidebarTriggerButtonRoot)
    expect(buttons).toHaveLength(2)
    let b0 = buttons.at(0) // Automatic resource with pending changes
    let b1 = buttons.at(1) // Automatic resource, no pending changes

    expectClickable(b0, false)
    expectManualTriggerIcon(b0, false)
    expectIsQueued(b0, false)
    expectWithTooltip(b0, TriggerButtonTooltip.UpdateInProgOrPending)

    expectClickable(b1, true)
    expectManualTriggerIcon(b1, false)
    expectIsQueued(b1, false)
    expectWithTooltip(b1, TriggerButtonTooltip.ClickToForce)
  })

  it("trigger button not clickable if resource is building", () => {
    let res = oneResource() // by default this resource is in the process of building
    let items = [new SidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
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
    let res = oneResource()
    res.currentBuild = {}
    res.buildHistory = []
    res.lastDeployTime = ""
    res.hasPendingChanges = false
    res.pendingBuildSince = ""
    let items = [new SidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
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
    res.currentBuild = {}
    res.queued = true
    let items = [new SidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
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
    res.lastDeployTime = ""
    res.currentBuild = {}
    res.hasPendingChanges = false
    res.pendingBuildSince = ""
    let items = [new SidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let button = root.find(SidebarTriggerButtonRoot)
    expect(button).toHaveLength(1)

    expectClickable(button, true)
    expectManualTriggerIcon(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, TriggerButtonTooltip.ClickToForce)
  })

  it("shows trigger button for Tiltfile", () => {
    let res = oneResource()
    res.name = ResourceName.tiltfile
    res.isTiltfile = true
    res.currentBuild = {} // not currently building
    res.hasPendingChanges = false
    res.pendingBuildSince = "0001-01-01T00:00:00Z"

    let items = [new SidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
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
