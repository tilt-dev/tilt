import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import Sidebar, { SidebarItem } from "./Sidebar"
import { oneResourceView, twoResourceView } from "./testdata.test"
import { mount } from "enzyme"
import { ResourceView } from "./types"
import MockDate from "mockdate"

describe("sidebar", () => {
  beforeEach(() => {
    MockDate.set("2018-1-1")
  })
  afterEach(() => MockDate.reset())
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
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders warning", () => {
    let items = oneResourceView().Resources.map((res: any) => {
      res.BuildHistory[0].Error = ""
      res.BuildHistory[0].Warnings = ["warning"]
      return new SidebarItem(res)
    })
    let sidebar = mount(
      <MemoryRouter initialEntries={["/"]}>
        <Sidebar
          isClosed={false}
          items={items}
          selected=""
          toggleSidebar={null}
          resourceView={ResourceView.Log}
        />
      </MemoryRouter>
    )
    expect(sidebar.find("li Link.has-warnings")).toHaveLength(1)
  })

  it("renders list of resources", () => {
    let items = twoResourceView().Resources.map((res: any) => {
      res.BuildHistory[0].Error = ""
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
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders resources that haven't been built yet", () => {
    let items = twoResourceView().Resources.map((res: any) => {
      res.LastDeployTime = "0001-01-01T00:00:00Z"
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
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})
