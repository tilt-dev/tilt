import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { LocalStorageContextProvider } from "./LocalStorage"
import { SidebarOptionsSetter } from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView from "./SidebarItemView"
import { SidebarPinContextProvider } from "./SidebarPin"
import SidebarResources from "./SidebarResources"
import { oneResource, oneResourceTest, tiltfileResource } from "./testdata"
import { ResourceView } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

function assertSidebarItemsAndOptions(
  root: ReactWrapper,
  names: string[],
  expectShowResources: boolean,
  expectShowTests: boolean
) {
  let sidebar = root.find(SidebarResources)
  expect(sidebar).toHaveLength(1)

  let items = sidebar.find(SidebarItemView)
  expect(items).toHaveLength(names.length)

  for (let i = 0; i < names.length; i++) {
    expect(items.at(i).props().item.name).toEqual(names[i])
  }

  let optSetter = sidebar.find(SidebarOptionsSetter)
  expect(optSetter).toHaveLength(1)
  expect(optSetter.find("input#resources").props().checked).toEqual(
    expectShowResources
  )
  expect(optSetter.find("input#tests").props().checked).toEqual(expectShowTests)
}

function newSidebarForTest(): ReactWrapper {
  let items = [tiltfileResource(), oneResource(), oneResourceTest()].map(
    (r) => new SidebarItem(r)
  )
  const root = mount(
    <MemoryRouter>
      <LocalStorageContextProvider tiltfileKey={"test"}>
        <SidebarPinContextProvider>
          <SidebarResources
            items={items}
            selected={""}
            resourceView={ResourceView.Log}
            pathBuilder={pathBuilder}
          />
        </SidebarPinContextProvider>
      </LocalStorageContextProvider>
    </MemoryRouter>
  )
  return root
}

it("shows tests and resources by default", () => {
  let root = newSidebarForTest()
  assertSidebarItemsAndOptions(
    root,
    ["(Tiltfile)", "vigoda", "boop"],
    true,
    true
  )
})

it("hides resources when resources unchecked", () => {
  let root = newSidebarForTest()
  assertSidebarItemsAndOptions(
    root,
    ["(Tiltfile)", "vigoda", "boop"],
    true,
    true
  )

  root
    .find("input#resources")
    .simulate("change", { target: { checked: false } })
  assertSidebarItemsAndOptions(root, ["(Tiltfile)", "boop"], false, true)
})

it("hides tests when tests unchecked", () => {
  let root = newSidebarForTest()
  assertSidebarItemsAndOptions(
    root,
    ["(Tiltfile)", "vigoda", "boop"],
    true,
    true
  )

  root.find("input#tests").simulate("change", { target: { checked: false } })
  assertSidebarItemsAndOptions(root, ["(Tiltfile)", "vigoda"], true, false)
})

it("hides resources and tests when both unchecked", () => {
  let root = newSidebarForTest()
  assertSidebarItemsAndOptions(
    root,
    ["(Tiltfile)", "vigoda", "boop"],
    true,
    true
  )

  root
    .find("input#resources")
    .simulate("change", { target: { checked: false } })
  root.find("input#tests").simulate("change", { target: { checked: false } })
  assertSidebarItemsAndOptions(root, ["(Tiltfile)"], false, false)
})

it("doesn't show SidebarOptionSetter if no tests present", () => {
  let items = [tiltfileResource(), oneResource()].map((r) => new SidebarItem(r))
  const root = mount(
    <MemoryRouter>
      <LocalStorageContextProvider tiltfileKey={"test"}>
        <SidebarPinContextProvider>
          <SidebarResources
            items={items}
            selected={""}
            resourceView={ResourceView.Log}
            pathBuilder={pathBuilder}
          />
        </SidebarPinContextProvider>
      </LocalStorageContextProvider>
    </MemoryRouter>
  )
  let sidebar = root.find(SidebarResources)
  expect(sidebar).toHaveLength(1)

  let optSetter = sidebar.find(SidebarOptionsSetter)
  expect(optSetter).toHaveLength(0)
})
// TODO:
//   - hide/show a type doesn't affect pinned
//   - checkboxes for tests/resources don't show when no tests present
//   - if test present; hide/show tests/resources; and then test removed (e.g. commented
//     out of tiltfile) then we hide the check boxes AND ALSO reset filters to show everything
