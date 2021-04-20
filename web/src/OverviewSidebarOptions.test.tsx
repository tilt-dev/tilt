import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import {
  TestsWithErrors,
  TwoResources,
  TwoResourcesTwoTests,
} from "./OverviewResourceSidebar.stories"
import {
  AlertsOnTopToggle,
  ClearResourceNameFilterButton,
  FilterOptionList,
  OverviewSidebarOptions,
  ResourceNameFilterTextField,
  TestsHiddenToggle,
  TestsOnlyToggle,
} from "./OverviewSidebarOptions"
import SidebarItemView from "./SidebarItemView"
import SidebarResources, {
  defaultOptions,
  SidebarListSection,
} from "./SidebarResources"
import { StarredResourcesContextProvider } from "./StarredResourcesContext"
import { SidebarOptions } from "./types"

const sidebarOptionsAccessor = accessorsForTesting<SidebarOptions>(
  "sidebar_options"
)

export function assertSidebarItemsAndOptions(
  root: ReactWrapper,
  names: string[],
  expectTestsHidden: boolean,
  expectTestsOnly: boolean,
  expectAlertsOnTop: boolean,
  expectedResourceNameFilter?: string
) {
  let sidebar = root.find(SidebarResources)
  expect(sidebar).toHaveLength(1)

  // only check items in the "all resources" section, i.e. don't look at starred things
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
  if (expectedResourceNameFilter !== undefined) {
    expect(optSetter.find(ResourceNameFilterTextField).props().value).toEqual(
      expectedResourceNameFilter
    )
  }
}

function clickTestsHiddenControl(root: ReactWrapper) {
  root.find(TestsHiddenToggle).simulate("click")
}
function clickTestsOnlyControl(root: ReactWrapper) {
  root.find(TestsOnlyToggle).simulate("click")
}

const allNames = ["(Tiltfile)", "vigoda", "snack", "beep", "boop"]

describe("overview sidebar options", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
    jest.useFakeTimers()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    localStorage.clear()
    jest.useRealTimers()
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

    expectIncrs({
      name: "ui.web.testsHiddenToggle",
      tags: { action: "click", newTestsHiddenState: "true" },
    })

    // re-check and make sure everything is visible
    clickTestsHiddenControl(root)
    assertSidebarItemsAndOptions(root, allNames, false, false, false)
  })

  it("shows only tests when TestsOnly enabled", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, false, false, false)

    clickTestsOnlyControl(root)
    assertSidebarItemsAndOptions(root, ["beep", "boop"], false, true, false)

    expectIncrs({
      name: "ui.web.testsOnlyToggle",
      tags: { action: "click", newTestsOnlyState: "true" },
    })

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

  it("applies the name filter", () => {
    // 'B p' tests both case insensitivity and a multi-term query
    sidebarOptionsAccessor.set({ ...defaultOptions, resourceNameFilter: "B p" })
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <StarredResourcesContextProvider>
            {TwoResourcesTwoTests()}
          </StarredResourcesContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )

    assertSidebarItemsAndOptions(
      root,
      ["beep", "boop"],
      defaultOptions.testsHidden,
      defaultOptions.testsOnly,
      defaultOptions.alertsOnTop,
      "B p"
    )
  })

  it("says no matches found", () => {
    sidebarOptionsAccessor.set({
      ...defaultOptions,
      resourceNameFilter: "asdfawfwef",
    })
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <StarredResourcesContextProvider>
            {TwoResourcesTwoTests()}
          </StarredResourcesContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )

    const resourceSectionItems = root
      .find(SidebarListSection)
      .find({ name: "resources" })
      .find("li")
    expect(resourceSectionItems.map((n) => n.text())).toEqual([
      "No matching resources",
    ])
  })

  it("reports analytics, debounced, when search bar edited", () => {
    const root = mount(
      <OverviewSidebarOptions
        options={defaultOptions}
        setOptions={() => {}}
        showFilters={true}
      />
    )
    const tf = root.find(ResourceNameFilterTextField)
    // two changes in rapid succession should result in only one analytics event
    tf.props().onChange({ target: { value: "foo" } })
    tf.props().onChange({ target: { value: "foobar" } })
    expectIncrs(...[])
    jest.runTimersToTime(10000)
    expectIncrs({ name: "ui.web.resourceNameFilter", tags: { action: "edit" } })
  })

  it("reports analytics when search bar cleared", () => {
    const root = mount(
      <OverviewSidebarOptions
        options={{ ...defaultOptions, resourceNameFilter: "foo" }}
        setOptions={() => {}}
        showFilters={true}
      />
    )
    const button = root.find(ClearResourceNameFilterButton)

    button.simulate("click")
    expectIncrs({
      name: "ui.web.clearResourceNameFilter",
      tags: { action: "click" },
    })
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
