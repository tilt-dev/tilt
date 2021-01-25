import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { LocalStorageContextProvider, makeKey } from "./LocalStorage"
import OverviewItemView from "./OverviewItemView"
import OverviewPane, {
  AlertResources,
  AllResources,
  PinnedResources,
  TestResources,
} from "./OverviewPane"
import { TwoResources } from "./OverviewPane.stories"
import { SidebarPinContextProvider } from "./SidebarPin"
import {
  oneResourceBuilding,
  oneResourceNoAlerts,
  oneResourceTest,
  twoResourceView,
} from "./testdata"

function assertContainerWithResources(
  root: ReactWrapper,
  sel: any,
  names: string[] // pass empty list to assert that there are no resource cards in the container
) {
  let resourceContainer = root.find(sel)
  expect(resourceContainer).toHaveLength(1)

  let items = resourceContainer.find(OverviewItemView)
  expect(items).toHaveLength(names.length)
  for (let i = 0; i < names.length; i++) {
    expect(items.at(i).props().item.name).toEqual(names[i])
  }
}

it("renders all resources if no pinned and no tests", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>{TwoResources()}</MemoryRouter>
  )

  assertContainerWithResources(root, PinnedResources, [])
  assertContainerWithResources(root, AllResources, ["vigoda", "snack"])
  assertContainerWithResources(root, TestResources, [])
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

  assertContainerWithResources(root, PinnedResources, ["snack"])
  assertContainerWithResources(root, AllResources, ["vigoda", "snack"])
  assertContainerWithResources(root, TestResources, [])
})

it("renders test resources separate from all resources", () => {
  let view = twoResourceView()
  view.resources.push(oneResourceTest())

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      {<OverviewPane view={view} />}
    </MemoryRouter>
  )

  assertContainerWithResources(root, PinnedResources, [])
  assertContainerWithResources(root, AllResources, ["vigoda", "snack"])
  assertContainerWithResources(root, TestResources, ["boop"])
})

it("renders errors and warnings", () => {
  let view = twoResourceView()
  let foo = oneResourceNoAlerts()
  foo.name = "foo"
  view.resources.push(foo)
  let bar = oneResourceBuilding()[0]
  bar.name = "bar"
  view.resources.push(bar)

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      {<OverviewPane view={view} />}
    </MemoryRouter>
  )

  assertContainerWithResources(root, PinnedResources, [])
  assertContainerWithResources(root, AlertResources, ["vigoda", "snack"])
  assertContainerWithResources(root, AllResources, [
    "vigoda",
    "snack",
    "foo",
    "bar",
  ])
  assertContainerWithResources(root, TestResources, [])
})
