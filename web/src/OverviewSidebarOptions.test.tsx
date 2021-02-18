import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import {
  TestsWithErrors,
  TwoResources,
  TwoResourcesTwoTests,
} from "./OverviewResourceSidebar.stories"
import {
  AlertsOnTopToggle,
  FilterOptionList,
  OverviewSidebarOptions,
  TestsHiddenToggle,
  TestsOnlyToggle,
} from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import SidebarItemView from "./SidebarItemView"
import { SidebarPinContextProvider } from "./SidebarPin"
import SidebarResources, {
  defaultOptions,
  SidebarListSection,
} from "./SidebarResources"
import { SidebarOptions } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

const sidebarOptionsAccessor = accessorsForTesting<SidebarOptions>(
  "sidebar_options"
)
const pinnedResourcesAccessor = accessorsForTesting<string[]>(
  "pinned-resources"
)

export function assertSidebarItemsAndOptions(
  root: ReactWrapper,
  names: string[],
  expectTestsHidden: boolean,
  expectTestsOnly: boolean,
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
  expect(optSetter.find(TestsHiddenToggle).hasClass("is-enabled")).toEqual(
    expectTestsHidden
  )
  expect(optSetter.find(TestsOnlyToggle).hasClass("is-enabled")).toEqual(
    expectTestsOnly
  )
  expect(optSetter.find(AlertsOnTopToggle).hasClass("is-enabled")).toEqual(
    expectAlertsOnTop
  )
}

function clickTestsHiddenControl(root: ReactWrapper) {
  root.find(TestsHiddenToggle).simulate("click")
}
function clickTestsOnlyControl(root: ReactWrapper) {
  root.find(TestsOnlyToggle).simulate("click")
}

const allNames = ["(Tiltfile)", "vigoda", "snack", "beep", "boop"]

describe("overview sidebar options", () => {
  afterEach(() => {
    localStorage.clear()
  })

  it("shows all resources by default", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, false, false, false)
  })

  it("hides tests when TestsHidden enabled", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, false, false, false)

    clickTestsHiddenControl(root)
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack"],
      true,
      false,
      false
    )

    // re-check and make sure everything is visible
    clickTestsHiddenControl(root)
    assertSidebarItemsAndOptions(root, allNames, false, false, false)
  })

  it("shows only tests when TestsOnly enabled", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, false, false, false)

    clickTestsOnlyControl(root)
    assertSidebarItemsAndOptions(root, ["beep", "boop"], false, true, false)

    // re-check and make sure tests are visible
    clickTestsOnlyControl(root)
    assertSidebarItemsAndOptions(root, allNames, false, false, false)
  })

  it("only one filter option can be enabled at once", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, false, false, false)

    clickTestsHiddenControl(root)
    clickTestsOnlyControl(root)
    assertSidebarItemsAndOptions(root, ["beep", "boop"], false, true, false)

    // Make sure it works in the other direction too
    clickTestsHiddenControl(root)
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack"],
      true,
      false,
      false
    )
  })

  it("doesn't show filter options if no tests present", () => {
    const root = mount(TwoResources())

    let sidebar = root.find(SidebarResources)
    expect(sidebar).toHaveLength(1)

    let filters = sidebar.find(FilterOptionList)
    expect(filters).toHaveLength(0)
  })

  it("shows filter options when no tests are present if filter options are non-default", () => {
    sidebarOptionsAccessor.set({ ...defaultOptions, testsHidden: true })
    const root = mount(
      <tiltfileKeyContext.Provider value="test">
        {TwoResources()}
      </tiltfileKeyContext.Provider>
    )

    let sidebar = root.find(SidebarResources)
    expect(sidebar).toHaveLength(1)

    let filters = sidebar.find(FilterOptionList)
    expect(filters).toHaveLength(1)
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
      false,
      false,
      false
    )

    let pinned = root
      .find(SidebarListSection)
      .find({ name: "Pinned" })
      .find(SidebarItemView)
    expect(pinned).toHaveLength(1)
    expect(pinned.at(0).props().item.name).toEqual("beep")

    clickTestsHiddenControl(root)
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
  assertSidebarItemsAndOptions(root, origOrder, false, false, false)

  let aotToggle = root.find(AlertsOnTopToggle)
  aotToggle.simulate("click")

  assertSidebarItemsAndOptions(root, alertsOnTopOrder, false, false, true)

  aotToggle.simulate("click")
  assertSidebarItemsAndOptions(root, origOrder, false, false, false)
})
