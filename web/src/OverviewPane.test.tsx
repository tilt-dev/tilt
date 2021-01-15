import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { LocalStorageContextProvider, makeKey } from "./LocalStorage"
import OverviewItemView from "./OverviewItemView"
import OverviewPane from "./OverviewPane"
import { TwoResources } from "./OverviewPane.stories"
import { SidebarPinContextProvider } from "./SidebarPin"
import { oneResourceTest, twoResourceView } from "./testdata"

function assertContainerWithResources(
  root: ReactWrapper,
  className: string,
  names: string[]
) {
  let sel = ".resources-container." + className
  let resourceContainer = root.find(sel)
  expect(resourceContainer).toHaveLength(1)

  let items = resourceContainer.find(OverviewItemView)
  expect(items).toHaveLength(names.length)
  for (let i = 0; i < names.length; i++) {
    expect(items.at(i).props().item.name).toEqual(names[i])
  }
}

function assertContainerDNE(root: ReactWrapper, className: string) {
  let sel = ".resources-container." + className
  expect(root.find(sel)).toHaveLength(0)
}

it("renders all resources if no pinned and no tests", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>{TwoResources()}</MemoryRouter>
  )

  assertContainerDNE(root, "pinned")
  assertContainerWithResources(root, "all", ["vigoda", "snack"])
  assertContainerDNE(root, "tests")
})

it("renders pinned resources", () => {
  localStorage.setItem(
    makeKey("test", "pinned-resources"),
    JSON.stringify(["snack"])
  )

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <LocalStorageContextProvider tiltfileKey={"test"}>
        <SidebarPinContextProvider>{TwoResources()}</SidebarPinContextProvider>
      </LocalStorageContextProvider>
    </MemoryRouter>
  )

  assertContainerWithResources(root, "pinned", ["snack"])

  assertContainerWithResources(root, "all", ["vigoda", "snack"])
  assertContainerDNE(root, "tests")
})

it("renders test resources separate from all resources", () => {
  let view = twoResourceView()
  view.resources.push(oneResourceTest())

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      {<OverviewPane view={view} />}
    </MemoryRouter>
  )

  assertContainerDNE(root, "pinned")
  assertContainerWithResources(root, "all", ["vigoda", "snack"])
  assertContainerWithResources(root, "tests", ["boop"])
})
