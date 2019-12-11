import React from "react"
import { mount } from "enzyme"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import Sidebar, { SidebarItem } from "./Sidebar"
import { oneResource, twoResourceView } from "./testdata"
import { ResourceView, TriggerMode } from "./types"
import PathBuilder from "./PathBuilder"

type Resource = Proto.webviewResource

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
      let history = res.buildHistory ?? []
      history[0].error = "error!"
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
})
