import React from "react"
import { mount } from "enzyme"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import Sidebar, { SidebarItem } from "./Sidebar"
import { oneResource, twoResourceView } from "./testdata"
import { Resource, ResourceView, TriggerMode } from "./types"
import PathBuilder from "./PathBuilder"

let pathBuilder = new PathBuilder("localhost", "/")

let realDateNow = Date.now

describe("sidebar", () => {
  beforeEach(() => {
    Date.now = jest.fn(() => Date.UTC(2017, 11, 21, 15, 36, 7, 0))
  })
  afterEach(() => {
    Date.now = realDateNow
  })
  it("renders empty resource list without crashing", () => {
    const tree = renderer
      .create(
        <MemoryRouter initialEntries={["/"]}>
          <Sidebar
            isClosed={true}
            items={[]}
            selected=""
            toggleSidebar={null}
            resourceView={ResourceView.Log}
            pathBuilder={pathBuilder}
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders list of resources", () => {
    let items = twoResourceView().resources.map((res: Resource) => {
      res.buildHistory[0].error = "error!"
      return new SidebarItem(res)
    })
    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("abbreviates durations under a minute", () => {
    let items = [4, 9, 19, 29, 39, 49, 54].map(d => {
      let res = oneResource()
      res.name = `resource${d}`
      res.lastDeployTime = new Date(Date.now() - d * 1000).toISOString()
      return new SidebarItem(res)
    })

    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders resources that haven't been built yet", () => {
    let items = twoResourceView().resources.map((res: any) => {
      // currently building, no completed builds
      res.lastDeployTime = "0001-01-01T00:00:00Z"
      return new SidebarItem(res)
    })
    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("shows ready + dirty trigger button for manual resource with pending changes", () => {
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

    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("never shows dirty trigger button for automatic resources", () => {
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

    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("trigger button not ready if resource is building", () => {
    let res = oneResource() // by default this resource is in the process of building
    let items = [new SidebarItem(res)]

    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("trigger button not ready if resource waiting for first build", () => {
    let res = oneResource()
    res.currentBuild = {}
    res.lastDeployTime = "0001-01-01T00:00:00Z"
    let items = [new SidebarItem(res)]

    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders queued resource with class .isQueued and NOT .isReady", () => {
    let res = oneResource()
    res.currentBuild = {}
    res.queued = true
    let items = [new SidebarItem(res)]

    const tree = renderer
      .create(
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
      .toJSON()

    expect(tree).toMatchSnapshot()
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

    let element = root.find(".SidebarTriggerButton")
    expect(element).toHaveLength(1)
    expect(element.hasClass("clickable")).toBeFalsy()
    expect(element.hasClass("clickMe")).toBeFalsy()
    expect(element.hasClass("isSelected")).toBeFalsy()
    expect(element.hasClass("isQueued")).toBeFalsy()
    expect(element.prop("disabled")).toBeTruthy()
    expect(element.prop("title")).toContain("Tiltfile")
  })
})
