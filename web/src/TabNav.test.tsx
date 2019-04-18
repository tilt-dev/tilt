import React from "react"
import renderer from "react-test-renderer"
import TabNav from "./TabNav"
import { MemoryRouter } from "react-router"
import { oneResourceView, twoResourceView } from "./testdata.test"
import { SidebarItem } from "./Sidebar"
import { ResourceView } from "./types"

it("doesn't crash with empty resource list", () => {
  let sidebarItems: Array<SidebarItem> = []
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          resourceView={ResourceView.Log}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("previews resources", () => {
  let sidebarItems: Array<SidebarItem> = []
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          resourceView={ResourceView.Preview}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
