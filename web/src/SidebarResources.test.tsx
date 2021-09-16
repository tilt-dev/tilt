import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import LogStore from "./LogStore"
import { AlertsOnTopToggle } from "./OverviewSidebarOptions"
import { assertSidebarItemsAndOptions } from "./OverviewSidebarOptions.test"
import PathBuilder from "./PathBuilder"
import {
  DEFAULT_OPTIONS,
  ResourceListOptions,
  ResourceListOptionsContextProvider,
  RESOURCE_LIST_OPTIONS_KEY,
} from "./ResourceListOptionsContext"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import { StarredResourcesContextProvider } from "./StarredResourcesContext"
import StarResourceButton from "./StarResourceButton"
import {
  oneResource,
  oneResourceTestWithName,
  twoResourceView,
} from "./testdata"
import { ResourceView } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

const resourceListOptionsAccessor = accessorsForTesting<ResourceListOptions>(
  RESOURCE_LIST_OPTIONS_KEY
)
const starredItemsAccessor = accessorsForTesting<string[]>("pinned-resources")

function clickStar(
  root: ReactWrapper<any, React.Component["state"], React.Component>,
  name: string
) {
  let starButtons = root.find(StarResourceButton).find({ resourceName: name })
  expect(starButtons.length).toBeGreaterThan(0)
  starButtons.at(0).simulate("click")
}

describe("SidebarResources", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    localStorage.clear()
  })

  it("adds items to the starred list when items are starred", () => {
    let ls = new LogStore()
    let items = twoResourceView().uiResources.map((r) => new SidebarItem(r, ls))
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <StarredResourcesContextProvider>
            <ResourceListOptionsContextProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.Log}
                pathBuilder={pathBuilder}
                resourceListOptions={DEFAULT_OPTIONS}
              />
            </ResourceListOptionsContextProvider>
          </StarredResourcesContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )

    clickStar(root, "snack")

    expectIncrs(
      {
        name: "ui.web.star",
        tags: { starCount: "0", action: AnalyticsAction.Load },
      },
      {
        name: "ui.web.sidebarStarButton",
        tags: { action: AnalyticsAction.Click, newStarState: "true" },
      },
      {
        name: "ui.web.star",
        tags: { starCount: "1", action: AnalyticsAction.Star },
      }
    )

    expect(starredItemsAccessor.get()).toEqual(["snack"])
  })

  it("removes items from the starred list when items are unstarred", () => {
    let ls = new LogStore()
    let items = twoResourceView().uiResources.map((r) => new SidebarItem(r, ls))
    starredItemsAccessor.set(items.map((i) => i.name))

    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <StarredResourcesContextProvider>
            <ResourceListOptionsContextProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.Log}
                pathBuilder={pathBuilder}
                resourceListOptions={DEFAULT_OPTIONS}
              />
            </ResourceListOptionsContextProvider>
          </StarredResourcesContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )

    clickStar(root, "snack")

    expectIncrs(
      {
        name: "ui.web.star",
        tags: { starCount: "2", action: AnalyticsAction.Load },
      },
      {
        name: "ui.web.sidebarStarButton",
        tags: { action: AnalyticsAction.Click, newStarState: "false" },
      },
      {
        name: "ui.web.star",
        tags: { starCount: "1", action: AnalyticsAction.Unstar },
      }
    )

    expect(starredItemsAccessor.get()).toEqual(["vigoda"])
  })

  const loadCases: [string, ResourceListOptions, string[]][] = [
    [
      "alertsOnTop",
      { ...DEFAULT_OPTIONS, alertsOnTop: true },
      ["vigoda", "a", "b"],
    ],
    [
      "resourceNameFilter",
      { ...DEFAULT_OPTIONS, resourceNameFilter: "vig" },
      ["vigoda"],
    ],
  ]
  test.each(loadCases)(
    "loads %p from localStorage",
    (name, options, expectedItems) => {
      resourceListOptionsAccessor.set(options)

      let ls = new LogStore()
      const items = [
        oneResource(),
        oneResourceTestWithName("a"),
        oneResourceTestWithName("b"),
      ].map((res) => new SidebarItem(res, ls))

      const root = mount(
        <MemoryRouter>
          <tiltfileKeyContext.Provider value="test">
            <ResourceListOptionsContextProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.OverviewDetail}
                pathBuilder={pathBuilder}
                resourceListOptions={options}
              />
            </ResourceListOptionsContextProvider>
          </tiltfileKeyContext.Provider>
        </MemoryRouter>
      )

      assertSidebarItemsAndOptions(
        root,
        expectedItems,
        options.alertsOnTop,
        options.resourceNameFilter
      )
    }
  )

  const saveCases: [string, ResourceListOptions][] = [
    ["alertsOnTop", { ...DEFAULT_OPTIONS, alertsOnTop: true }],
    ["resourceNameFilter", { ...DEFAULT_OPTIONS, resourceNameFilter: "foo" }],
  ]
  test.each(saveCases)(
    "saves option %s to localStorage",
    (name, expectedOptions) => {
      let ls = new LogStore()
      const items = [
        oneResource(),
        oneResourceTestWithName("a"),
        oneResourceTestWithName("b"),
      ].map((res) => new SidebarItem(res, ls))

      const root = mount(
        <MemoryRouter>
          <tiltfileKeyContext.Provider value="test">
            <ResourceListOptionsContextProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.OverviewDetail}
                pathBuilder={pathBuilder}
                resourceListOptions={DEFAULT_OPTIONS}
              />
            </ResourceListOptionsContextProvider>
          </tiltfileKeyContext.Provider>
        </MemoryRouter>
      )

      let aotToggle = root.find(AlertsOnTopToggle)
      if (aotToggle.hasClass("is-enabled") !== expectedOptions.alertsOnTop) {
        aotToggle.simulate("click")
      }

      if (expectedOptions.resourceNameFilter.length) {
        root.find("input").simulate("change", {
          target: { value: expectedOptions.resourceNameFilter },
        })
      }

      const observedOptions = resourceListOptionsAccessor.get()
      expect(observedOptions).toEqual(expectedOptions)
    }
  )
})
