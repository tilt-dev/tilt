import { mount, ReactWrapper } from "enzyme"
import fetchMock from "jest-fetch-mock"
import React from "react"
import { MemoryRouter } from "react-router"
import { expectIncr } from "./analytics_test_helpers"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import { AlertsOnTopToggle } from "./OverviewSidebarOptions"
import { assertSidebarItemsAndOptions } from "./OverviewSidebarOptions.test"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import { SidebarItemBox } from "./SidebarItemView"
import { SidebarPinContextProvider } from "./SidebarPin"
import SidebarPinButton from "./SidebarPinButton"
import SidebarResources, { SidebarListSection } from "./SidebarResources"
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

function getPinnedItemNames(
  root: ReactWrapper<any, React.Component["state"], React.Component>
): Array<string> {
  let pinnedItems = root
    .find(SidebarListSection)
    .find({ name: "Pinned" })
    .find(SidebarItemBox)
  return pinnedItems.map((i) => i.prop("data-name"))
}

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
    fetchMock.resetMocks()
    fetchMock.mockResponse(JSON.stringify({}))
  })

  afterEach(() => {
    fetchMock.resetMocks()
    localStorage.clear()
  })

  it("adds items to the pinned group when items are pinned", () => {
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

    expect(getPinnedItemNames(root)).toEqual([])

    clickPin(root, "snack")

    expect(getPinnedItemNames(root)).toEqual(["snack"])

    expectIncr(0, "ui.web.pin", { pinCount: "0", action: "load" })
    expectIncr(1, "ui.web.pin", { pinCount: "1", action: "pin" })

    expect(pinnedItemsAccessor.get()).toEqual(["snack"])
  })

  it("reads pinned items from local storage", () => {
    pinnedItemsAccessor.set(["vigoda", "snack"])

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

    expect(getPinnedItemNames(root)).toEqual(["vigoda", "snack"])
  })

  it("removes items from the pinned group when items are pinned", () => {
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

    expect(getPinnedItemNames(root)).toEqual(["vigoda", "snack"])

    clickPin(root, "snack")

    expect(getPinnedItemNames(root)).toEqual(["vigoda"])

    expectIncr(0, "ui.web.pin", { pinCount: "2", action: "load" })
    expectIncr(1, "ui.web.pin", { pinCount: "1", action: "unpin" })

    expect(pinnedItemsAccessor.get()).toEqual(["vigoda"])
  })

  const loadCases: [string, SidebarOptions, string[]][] = [
    [
      "showResources",
      { showResources: false, showTests: true, alertsOnTop: false },
      ["a", "b"],
    ],
    [
      "showTests",
      { showResources: true, showTests: false, alertsOnTop: false },
      ["vigoda"],
    ],
    [
      "alertsOnTop",
      { showResources: true, showTests: true, alertsOnTop: true },
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
        options.showResources,
        options.showTests,
        options.alertsOnTop
      )
    }
  )

  const saveCases: [string, SidebarOptions][] = [
    [
      "showResources",
      { showResources: false, showTests: true, alertsOnTop: true },
    ],
    ["showTests", { showResources: true, showTests: false, alertsOnTop: true }],
    [
      "alertsOnTop",
      { showResources: true, showTests: true, alertsOnTop: false },
    ],
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

      let resToggle = root.find("input#resources")
      if (resToggle.props().checked != expectedOptions.showResources) {
        resToggle.simulate("click")
      }

      let testToggle = root.find("input#tests")
      if (testToggle.props().checked != expectedOptions.showTests) {
        testToggle.simulate("click")
      }

      let aotToggle = root.find(AlertsOnTopToggle)
      if (aotToggle.hasClass("is-enabled") != expectedOptions.alertsOnTop) {
        aotToggle.simulate("click")
      }

      const observedOptions = sidebarOptionsAccessor.get()
      expect(observedOptions).toEqual(expectedOptions)
    }
  )
})
