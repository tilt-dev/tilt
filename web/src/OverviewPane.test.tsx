import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import OverviewItemView from "./OverviewItemView"
import OverviewPane, { AllResources, TestResources } from "./OverviewPane"
import { TwoResources } from "./OverviewPane.stories"
import { StarredResourceLabel } from "./StarredResourceBar"
import { StarredResourcesContextProvider } from "./StarredResourcesContext"
import { oneResourceTest, twoResourceView } from "./testdata"

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

function assertStarredResources(root: ReactWrapper, names: string[]) {
  const renderedStarredResourceNames = root
    .find(StarredResourceLabel)
    .map((i) => i.text())
  expect(renderedStarredResourceNames).toEqual(names)
}

const starredResourcesAccessor = accessorsForTesting<string[]>(
  "pinned-resources"
)

it("renders all resources if no starred and no tests", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>{TwoResources()}</MemoryRouter>
  )

  assertContainerWithResources(root, AllResources, ["vigoda", "snack"])
  assertContainerWithResources(root, TestResources, [])
  assertStarredResources(root, [])
})

it("renders starred resources", () => {
  starredResourcesAccessor.set(["snack"])

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <tiltfileKeyContext.Provider value="test">
        <StarredResourcesContextProvider>
          {TwoResources()}
        </StarredResourcesContextProvider>
      </tiltfileKeyContext.Provider>
    </MemoryRouter>
  )

  assertContainerWithResources(root, AllResources, ["vigoda", "snack"])
  assertContainerWithResources(root, TestResources, [])
  assertStarredResources(root, ["snack"])
})

it("renders test resources separate from all resources", () => {
  let view = twoResourceView()
  view.resources.push(oneResourceTest())

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      {<OverviewPane view={view} />}
    </MemoryRouter>
  )

  assertContainerWithResources(root, AllResources, ["vigoda", "snack"])
  assertContainerWithResources(root, TestResources, ["boop"])
  assertStarredResources(root, [])
})
