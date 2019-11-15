import React from "react"
import { mount } from "enzyme"
import SidebarTriggerButton, {
  TriggerButtonTooltip,
} from "./SidebarTriggerButton"
import { Resource, ResourceView, TriggerMode } from "./types"
import { oneResource, twoResourceView } from "./testdata"
import Sidebar, { SidebarItem } from "./Sidebar"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"

let pathBuilder = new PathBuilder("localhost", "/")

let assertTriggerButtonProps = (
  button: any,
  expectClickable: boolean,
  expectClickMe: boolean,
  expectIsSelected: boolean,
  expectIsQueued: boolean,
  expectDisabled: boolean,
  expectTooltip: string
) => {
  expect(button.hasClass("clickable")).toEqual(expectClickable)
  expect(button.hasClass("clickMe")).toEqual(expectClickMe)
  expect(button.hasClass("isSelected")).toEqual(expectIsSelected)
  expect(button.hasClass("isQueued")).toEqual(expectIsQueued)
  expect(button.prop("disabled")).toEqual(expectDisabled)
  expect(button.prop("title")).toEqual(expectTooltip)
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

    let element = root.find(".SidebarTriggerButton")
    expect(element).toHaveLength(1)
    element.simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(1)
    expect(fetchMock.mock.calls[0][0]).toEqual("//localhost/api/trigger")
    expect(fetchMock.mock.calls[0][1].method).toEqual("post")
    expect(fetchMock.mock.calls[0][1].body).toEqual(
      JSON.stringify({ manifest_names: ["doggos"] })
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

    let element = root.find(".SidebarTriggerButton")
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

    let element = root.find(".SidebarTriggerButton")
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

    let buttons = root.find(".SidebarTriggerButton")
    expect(buttons).toHaveLength(2)

    // Manual resource with pending changes
    assertTriggerButtonProps(
      buttons.at(0),
      true,
      true,
      false,
      false,
      false,
      TriggerButtonTooltip.ManualResourcePendingChanges
    )

    // Manual resource, no pending changes
    assertTriggerButtonProps(
      buttons.at(1),
      true,
      false,
      false,
      false,
      false,
      TriggerButtonTooltip.ClickToForce
    )
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

    let buttons = root.find(".SidebarTriggerButton")
    expect(buttons).toHaveLength(2)

    // Automatic resource with pending changes -- !.clickMe, !.clickable
    assertTriggerButtonProps(
      buttons.at(0),
      false,
      false,
      false,
      false,
      true,
      TriggerButtonTooltip.UpdateInProgOrPending
    )

    // Automatic resource, no pending changes
    assertTriggerButtonProps(
      buttons.at(1),
      true,
      false,
      false,
      false,
      false,
      TriggerButtonTooltip.ClickToForce
    )
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

    let button = root.find(".SidebarTriggerButton")
    expect(button).toHaveLength(1)

    assertTriggerButtonProps(
      button,
      false,
      false,
      false,
      false,
      true,
      TriggerButtonTooltip.UpdateInProgOrPending
    )
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

    let button = root.find(".SidebarTriggerButton")
    expect(button).toHaveLength(1)

    assertTriggerButtonProps(
      button,
      false,
      false,
      false,
      false,
      true,
      TriggerButtonTooltip.UpdateInProgOrPending
    )
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

    let button = root.find(".SidebarTriggerButton")
    expect(button).toHaveLength(1)

    assertTriggerButtonProps(
      button,
      false,
      false,
      false,
      true,
      true,
      TriggerButtonTooltip.AlreadyQueued
    )
  })

  it("shows a trigger button for resource that failed its initial build", () => {
    let res = oneResource()
    res.lastDeployTime = ""
    res.currentBuild = false
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

    let button = root.find(".SidebarTriggerButton")
    expect(button).toHaveLength(1)

    assertTriggerButtonProps(
      button,
      true,
      false,
      false,
      false,
      false,
      TriggerButtonTooltip.ClickToForce
    )
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

    let button = root.find(".SidebarTriggerButton")
    expect(button).toHaveLength(1)
    assertTriggerButtonProps(
      button,
      false,
      false,
      false,
      false,
      true,
      TriggerButtonTooltip.CannotTriggerTiltfile
    )
  })
})
