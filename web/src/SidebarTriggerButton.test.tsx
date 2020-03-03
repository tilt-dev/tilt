import React from "react"
import { mount } from "enzyme"
import SidebarTriggerButton, {
  TriggerButtonTooltip,
} from "./SidebarTriggerButton"
import { ResourceView, TriggerMode } from "./types"
import { oneResource, twoResourceView } from "./testdata"
import Sidebar, { SidebarItem } from "./Sidebar"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"

type Resource = Proto.webviewResource

let pathBuilder = new PathBuilder("localhost", "/")

let expectClickable = (button: any, expected: boolean) => {
  expect(button.hasClass("clickable")).toEqual(expected)
  expect(button.prop("disabled")).toEqual(!expected)
}
let expectClickMe = (button: any, expected: boolean) => {
  expect(button.hasClass("clickMe")).toEqual(expected)
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
    fetchMock.resetMocks()
  })

  it("POSTs to endpoint when clicked", () => {
    fetchMock.mockResponse(JSON.stringify({}))

    const root = mount(
      <SidebarTriggerButton
        isTiltfile={false}
        isSelected={true}
        resourceName="doggos"
        triggerMode={TriggerMode.TriggerModeManualAfterInitial}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
      />
    )

    let element = root.find("button.SidebarTriggerButton")
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(1)
    expect(fetchMock.mock.calls[0][0]).toEqual("//localhost/api/trigger")
    expect(fetchMock.mock.calls[0][1].method).toEqual("post")
    expect(fetchMock.mock.calls[0][1].body).toEqual(
      JSON.stringify({
        manifest_names: ["doggos"],
        build_reason: 16 /* BuildReasonFlagTriggerWeb */,
      })
    )
  })

  it("disables button when resource is queued", () => {
    fetchMock.mockResponse(JSON.stringify({}))

    const root = mount(
      <SidebarTriggerButton
        isTiltfile={false}
        isSelected={true}
        resourceName="doggos"
        triggerMode={TriggerMode.TriggerModeManualAfterInitial}
        hasBuilt={true}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={true}
      />
    )

    let element = root.find("button.SidebarTriggerButton")
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(0)
  })

  it("shows the button for TriggerModeManualIncludingInitial", () => {
    fetchMock.mockResponse(JSON.stringify({}))

    const root = mount(
      <SidebarTriggerButton
        isSelected={true}
        isTiltfile={false}
        resourceName="doggos"
        triggerMode={TriggerMode.TriggerModeManualIncludingInitial}
        hasBuilt={false}
        isBuilding={false}
        hasPendingChanges={false}
        isQueued={false}
      />
    )

    let element = root.find("button.SidebarTriggerButton")
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(1)
  })

  it("shows clickable + clickMe trigger button for manual resource with pending changes", () => {
    let items = twoResourceView().resources.map((res: Resource, i: number) => {
      res.triggerMode = TriggerMode.TriggerModeManualAfterInitial // both manual
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
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let buttons = root.find("button.SidebarTriggerButton")
    expect(buttons).toHaveLength(2)

    let b0 = buttons.at(0) // Manual resource with pending changes
    let b1 = buttons.at(1) // Manual resource, no pending changes

    expectClickable(b0, true)
    expectClickMe(b0, true)
    expectIsQueued(b0, false)
    expectWithTooltip(b0, TriggerButtonTooltip.ManualResourcePendingChanges)

    expectClickable(b1, true)
    expectClickMe(b1, false)
    expectIsQueued(b1, false)
    expectWithTooltip(b1, TriggerButtonTooltip.ClickToForce)
  })

  it("shows selected trigger button for selected resource", () => {
    let items = twoResourceView().resources.map((res: Resource, i: number) => {
      res.triggerMode = TriggerMode.TriggerModeManualAfterInitial // both manual
      res.currentBuild = {} // not currently building
      if (i == 0) {
        res.name = "selected resource"
      }

      return new SidebarItem(res)
    })

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <Sidebar
          isClosed={false}
          items={items}
          selected="selected resource"
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let buttons = root.find("button.SidebarTriggerButton")
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
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let buttons = root.find("button.SidebarTriggerButton")
    expect(buttons).toHaveLength(2)
    let b0 = buttons.at(0) // Automatic resource with pending changes
    let b1 = buttons.at(1) // Automatic resource, no pending changes

    expectClickable(b0, false)
    expectClickMe(b0, false)
    expectIsQueued(b0, false)
    expectWithTooltip(b0, TriggerButtonTooltip.UpdateInProgOrPending)

    expectClickable(b1, true)
    expectClickMe(b1, false)
    expectIsQueued(b1, false)
    expectWithTooltip(b1, TriggerButtonTooltip.ClickToForce)
  })

  it("trigger button not clickable if resource is building", () => {
    let res = oneResource() // by default this resource is in the process of building
    let items = [new SidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let button = root.find("button.SidebarTriggerButton")
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectClickMe(button, false)
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
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let button = root.find("button.SidebarTriggerButton")
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectClickMe(button, false)
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
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let button = root.find("button.SidebarTriggerButton")
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectClickMe(button, false)
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
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let button = root.find("button.SidebarTriggerButton")
    expect(button).toHaveLength(1)

    expectClickable(button, true)
    expectClickMe(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, TriggerButtonTooltip.ClickToForce)
  })

  it("disables trigger button for Tiltfile", () => {
    let res = oneResource()
    res.name = "(Tiltfile)"
    res.isTiltfile = true
    res.currentBuild = {} // not currently building
    res.hasPendingChanges = false
    res.pendingBuildSince = "0001-01-01T00:00:00Z"

    let items = [new SidebarItem(res)]

    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let button = root.find("button.SidebarTriggerButton")
    expect(button).toHaveLength(1)

    expectClickable(button, false)
    expectClickMe(button, false)
    expectIsQueued(button, false)
    expectWithTooltip(button, TriggerButtonTooltip.CannotTriggerTiltfile)
  })
})
