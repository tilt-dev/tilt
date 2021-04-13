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
  AlertsOnTopToggle,
  ResourceNameFilterTextField,
  TestsHiddenToggle,
  TestsOnlyToggle,
} from "./OverviewSidebarOptions"
import { assertSidebarItemsAndOptions } from "./OverviewSidebarOptions.test"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import { SidebarPinContextProvider } from "./SidebarPin"
import SidebarPinButton from "./SidebarPinButton"
import SidebarResources from "./SidebarResources"
import {
  oneResource,
  oneResourceTestWithName,
  twoResourceView,
} from "./testdata"
import { ResourceView, SidebarOptions } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

const sidebarOptionsAccessor = accessorsForTesting<SidebarOptions>(
  "sidebar_options"
)
const pinnedItemsAccessor = accessorsForTesting<string[]>("pinned-resources")

function clickPin(
  root: ReactWrapper<any, React.Component["state"], React.Component>,
  name: string
) {
  let pinButtons = root.find(SidebarPinButton).find({ resourceName: name })
  expect(pinButtons.length).toBeGreaterThan(0)
  pinButtons.at(0).simulate("click")
}

describe("SidebarResources", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    localStorage.clear()
  })

  it("adds items to the pinned list when items are pinned", () => {
    let items = twoResourceView().resources.map((r) => new SidebarItem(r))
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

    clickPin(root, "snack")

    expectIncrs(
      { name: "ui.web.pin", tags: { pinCount: "0", action: "load" } },
      {
        name: "ui.web.sidebarPinButton",
        tags: { action: "click", newPinState: "true" },
      },
      { name: "ui.web.pin", tags: { pinCount: "1", action: "pin" } }
    )

    expect(pinnedItemsAccessor.get()).toEqual(["snack"])
  })

  it("removes items from the pinned list when items are unpinned", () => {
    let items = twoResourceView().resources.map((r) => new SidebarItem(r))
    pinnedItemsAccessor.set(items.map((i) => i.name))

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

    clickPin(root, "snack")

    expectIncrs(
      { name: "ui.web.pin", tags: { pinCount: "2", action: "load" } },
      {
        name: "ui.web.sidebarPinButton",
        tags: { action: "click", newPinState: "false" },
      },
      { name: "ui.web.pin", tags: { pinCount: "1", action: "unpin" } }
    )

    expect(pinnedItemsAccessor.get()).toEqual(["vigoda"])
  })

  const falseyOptions: SidebarOptions = {
    testsHidden: false,
    testsOnly: false,
    alertsOnTop: false,
    resourceNameFilter: "",
  }

  const loadCases: [string, any, string[]][] = [
    ["tests only", { ...falseyOptions, testsOnly: true }, ["a", "b"]],
    ["tests hidden", { ...falseyOptions, testsHidden: true }, ["vigoda"]],
    [
      "alertsOnTop",
      { ...falseyOptions, alertsOnTop: true },
      ["vigoda", "a", "b"],
    ],
    [
      "resourceNameFilter",
      { ...falseyOptions, resourceNameFilter: "vig" },
      ["vigoda"],
    ],
    [
      "resourceNameFilter undefined",
      { ...falseyOptions, resourceNameFilter: undefined },
      ["vigoda", "a", "b"],
    ],
  ]
  test.each(loadCases)(
    "loads %p from localStorage",
    (name, options, expectedItems) => {
      sidebarOptionsAccessor.set(options)

      const items = [
        oneResource(),
        oneResourceTestWithName("a"),
        oneResourceTestWithName("b"),
      ].map((res) => new SidebarItem(res))

      const root = mount(
        <MemoryRouter>
          <tiltfileKeyContext.Provider value="test">
            <SidebarResources
              items={items}
              selected={""}
              resourceView={ResourceView.OverviewDetail}
              pathBuilder={pathBuilder}
            />
          </tiltfileKeyContext.Provider>
        </MemoryRouter>
      )

      assertSidebarItemsAndOptions(
        root,
        expectedItems,
        options.testsHidden,
        options.testsOnly,
        options.alertsOnTop
      )
    }
  )

  const saveCases: [string, SidebarOptions][] = [
    ["testsHidden", { ...falseyOptions, testsHidden: true }],
    ["testsOnly", { ...falseyOptions, testsOnly: true }],
    ["alertsOnTop", { ...falseyOptions, alertsOnTop: true }],
    ["resourceNameFilter", { ...falseyOptions, resourceNameFilter: "foo" }],
  ]
  test.each(saveCases)(
    "saves option %s to localStorage",
    (name, expectedOptions) => {
      const items = [
        oneResource(),
        oneResourceTestWithName("a"),
        oneResourceTestWithName("b"),
      ].map((res) => new SidebarItem(res))

      const root = mount(
        <MemoryRouter>
          <tiltfileKeyContext.Provider value="test">
            <SidebarResources
              items={items}
              selected={""}
              resourceView={ResourceView.OverviewDetail}
              pathBuilder={pathBuilder}
            />
          </tiltfileKeyContext.Provider>
        </MemoryRouter>
      )

      let testsHiddenControl = root.find(TestsHiddenToggle)
      if (
        testsHiddenControl.hasClass("is-enabled") !==
        expectedOptions.testsHidden
      ) {
        testsHiddenControl.simulate("click")
      }

      let testsOnlyControl = root.find(TestsOnlyToggle)
      if (
        testsOnlyControl.hasClass("is-enabled") !== expectedOptions.testsOnly
      ) {
        testsOnlyControl.simulate("click")
      }

      let aotToggle = root.find(AlertsOnTopToggle)
      if (aotToggle.hasClass("is-enabled") !== expectedOptions.alertsOnTop) {
        aotToggle.simulate("click")
      }

      let resourceNameFilterTextField = root.find(ResourceNameFilterTextField)
      if (
        resourceNameFilterTextField.props().value !==
        expectedOptions.resourceNameFilter
      ) {
        resourceNameFilterTextField.find("input").simulate("change", {
          target: { value: expectedOptions.resourceNameFilter },
        })
      }

      const observedOptions = sidebarOptionsAccessor.get()
      expect(observedOptions).toEqual(expectedOptions)
    }
  )
})
