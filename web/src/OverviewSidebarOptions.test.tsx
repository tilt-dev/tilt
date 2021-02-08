import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import {
  TestsWithErrors,
  TwoResourcesTwoTests,
} from "./OverviewResourceSidebar.stories"
import {
  AlertsOnTopToggle,
  OverviewSidebarOptions,
} from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView from "./SidebarItemView"
import { SidebarPinContextProvider } from "./SidebarPin"
import SidebarResources, { SidebarListSection } from "./SidebarResources"
import { oneResource, tiltfileResource } from "./testdata"
import { ResourceView } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

const pinnedResourcesAccessor = accessorsForTesting<string[]>(
  "pinned-resources"
)

export function assertSidebarItemsAndOptions(
  root: ReactWrapper,
  names: string[],
  expectShowResources: boolean,
  expectShowTests: boolean,
  expectAlertsOnTop: boolean
) {
  let sidebar = root.find(SidebarResources)
  expect(sidebar).toHaveLength(1)

  // only check items in the "all resources" section, i.e. don't look at pinned things
  // or we'll have duplicates
  let all = sidebar.find(SidebarListSection).find({ name: "resources" })
  let items = all.find(SidebarItemView)
  const observedNames = items.map((i) => i.props().item.name)
  expect(observedNames).toEqual(names)

  let optSetter = sidebar.find(OverviewSidebarOptions)
  expect(optSetter).toHaveLength(1)
  expect(optSetter.find("input#resources").props().checked).toEqual(
    expectShowResources
  )
  expect(optSetter.find("input#tests").props().checked).toEqual(expectShowTests)
  expect(optSetter.find(AlertsOnTopToggle).hasClass("is-enabled")).toEqual(
    expectAlertsOnTop
  )
}

function clickResourcesToggle(root: ReactWrapper) {
  root.find("input#resources").simulate("click")
}
function clickTestsToggle(root: ReactWrapper) {
  root.find("input#tests").simulate("click")
}

const allNames = ["(Tiltfile)", "vigoda", "snack", "beep", "boop"]

describe("overview sidebar options", () => {
  afterEach(() => {
    localStorage.clear()
  })

  it("shows tests and resources by default", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true, false)
  })

  it("hides resources when resources unchecked", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true, false)

    clickResourcesToggle(root)
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "beep", "boop"],
      false,
      true,
      false
    )

    // re-check and make sure resources are visible
    clickResourcesToggle(root)
    assertSidebarItemsAndOptions(root, allNames, true, true, false)
  })

  it("hides tests when tests unchecked", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true, false)

    clickTestsToggle(root)
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack"],
      true,
      false,
      false
    )

    // re-check and make sure tests are visible
    clickTestsToggle(root)
    assertSidebarItemsAndOptions(root, allNames, true, true, false)
  })

  it("hides resources and tests when both unchecked", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true, false)

    clickResourcesToggle(root)
    clickTestsToggle(root)
    assertSidebarItemsAndOptions(root, ["(Tiltfile)"], false, false, false)
  })

  it("doesn't show SidebarOptionSetter if no tests present", () => {
    let items = [tiltfileResource(), oneResource()].map(
      (r) => new SidebarItem(r)
    )
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <SidebarPinContextProvider>
            <SidebarResources
              items={items}
              selected={""}
              resourceView={ResourceView.Log}
              pathBuilder={pathBuilder}
            />
          </SidebarPinContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )
    let sidebar = root.find(SidebarResources)
    expect(sidebar).toHaveLength(1)

    let optSetter = sidebar.find(OverviewSidebarOptions)
    expect(optSetter).toHaveLength(0)
  })

  it("still displays pinned tests when tests hidden", () => {
    pinnedResourcesAccessor.set(["beep"])
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <SidebarPinContextProvider>
            {TwoResourcesTwoTests()}
          </SidebarPinContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )

    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack", "beep", "boop"],
      true,
      true,
      false
    )

    let pinned = root
      .find(SidebarListSection)
      .find({ name: "Pinned" })
      .find(SidebarItemView)
    expect(pinned).toHaveLength(1)
    expect(pinned.at(0).props().item.name).toEqual("beep")

    clickTestsToggle(root)
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack"],
      true,
      false,
      false
    )

    // "beep" should still be pinned, even though we're no longer showing tests in the main resource list
    pinned = root
      .find(SidebarListSection)
      .find({ name: "Pinned" })
      .find(SidebarItemView)
    expect(pinned).toHaveLength(1)
    expect(pinned.at(0).props().item.name).toEqual("beep")
  })
})

it("toggles/untoggles Alerts On Top sorting when button clicked", () => {
  const root = mount(TestsWithErrors())

  const origOrder = [
    "(Tiltfile)",
    "test_0",
    "test_1",
    "test_2",
    "test_3",
    "test_4",
    "test_5",
    "test_6",
    "test_7",
  ]
  const alertsOnTopOrder = [
    "test_0",
    "test_2",
    "test_4",
    "test_6",
    "(Tiltfile)",
    "test_1",
    "test_3",
    "test_5",
    "test_7",
  ]
  assertSidebarItemsAndOptions(root, origOrder, true, true, false)

  let aotToggle = root.find(AlertsOnTopToggle)
  aotToggle.simulate("click")

  assertSidebarItemsAndOptions(root, alertsOnTopOrder, true, true, true)

  aotToggle.simulate("click")
  assertSidebarItemsAndOptions(root, origOrder, true, true, false)
})
