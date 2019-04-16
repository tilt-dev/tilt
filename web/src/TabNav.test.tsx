import React from "react"
import renderer from "react-test-renderer"
import TabNav from "./TabNav"
import { ResourceView } from "./HUD"
import { MemoryRouter } from "react-router"
import { oneResourceView, twoResourceView } from "./testdata.test"
import { SidebarItem } from "./Sidebar"

it("doesn't crash with empty resource list", () => {
  let sidebarItems = []
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          resourceName={"foo"}
          sidebarItems={sidebarItems}
          resourceView={ResourceView.Log}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("previews resources", () => {
  let sidebarItems = []
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          resourceName={"foo"}
          sidebarItems={sidebarItems}
          resourceView={ResourceView.Preview}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("previews the first resource that has an endpoint if nothing is selected", () => {
  let sidebarItems: Array<SidebarItem> = oneResourceView().Resources.map(
    (res: any) => {
      res.BuildHistory[0].Error = ""
      res.BuildHistory[0].Warnings = ["warning"]
      return new SidebarItem(res)
    }
  )

  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          resourceName={""}
          sidebarItems={sidebarItems}
          resourceView={ResourceView.Preview}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
