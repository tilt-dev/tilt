import React from "react"
import { mount } from "enzyme"
import renderer from "react-test-renderer"
import { MemoryRouter, useHistory } from "react-router"
import Sidebar, { SidebarItemLink } from "./SidebarResources"
import SidebarItem from "./SidebarItem"
import { oneResource, twoResourceView } from "./testdata"
import { ResourceView, TriggerMode } from "./types"
import PathBuilder from "./PathBuilder"
import SidebarResources from "./SidebarResources"
import fetchMock from "jest-fetch-mock"

type Resource = Proto.webviewResource

let pathBuilder = new PathBuilder("localhost", "/")

let realDateNow = Date.now

describe("sidebar", () => {
  beforeEach(() => {
    fetchMock.resetMocks()
    Date.now = jest.fn(() => Date.UTC(2017, 11, 21, 15, 36, 7, 0))
  })
  afterEach(() => {
    Date.now = realDateNow
  })
  it("renders empty resource list without crashing", () => {
    const tree = renderer
      .create(
        <MemoryRouter initialEntries={["/"]}>
          <SidebarResources
            items={[]}
            selected=""
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
      let history = res.buildHistory ?? []
      history[0].error = "error!"
      return new SidebarItem(res)
    })
    const tree = renderer
      .create(
        <MemoryRouter initialEntries={["/"]}>
          <SidebarResources
            items={items}
            selected=""
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
          <SidebarResources
            items={items}
            selected=""
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
          <SidebarResources
            items={items}
            selected=""
            resourceView={ResourceView.Log}
            pathBuilder={pathBuilder}
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("reloads selected resource on click", () => {
    let items = twoResourceView().resources.map(
      (res: any) => new SidebarItem(res)
    )
    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <SidebarResources
          items={items}
          selected={items[0].name}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let links = root.find(SidebarItemLink)
    expect(links).toHaveLength(3)

    links.at(1).simulate("click")

    expect(fetchMock.mock.calls.length).toEqual(1)
    expect(fetchMock.mock.calls[0][0]).toEqual("/api/trigger")
    expect(fetchMock.mock.calls[0][1]?.method).toEqual("post")
    expect(fetchMock.mock.calls[0][1]?.body).toEqual(
      JSON.stringify({
        manifest_names: ["vigoda"],
        build_reason: 16 /* BuildReasonFlagTriggerWeb */,
      })
    )

    root.unmount()
  })

  it("navigates to unselected resource on click", () => {
    let items = twoResourceView().resources.map(
      (res: any) => new SidebarItem(res)
    )
    let history: any
    let CaptureHistory = () => {
      history = useHistory()
      return <span />
    }
    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <CaptureHistory />
        <SidebarResources
          items={items}
          selected={items[0].name}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    let links = root.find(SidebarItemLink)
    expect(links).toHaveLength(3)
    expect(links.at(2).html()).toContain('href="/r/snack"')
    root.unmount()
  })
})
