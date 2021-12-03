import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { accessorsForTesting, tiltfileKeyContext } from "./BrowserStorage"
import Features, { FeaturesProvider, Flag } from "./feature"
import LogStore from "./LogStore"
import { AlertsOnTopToggle } from "./OverviewSidebarOptions"
import { assertSidebarItemsAndOptions } from "./OverviewSidebarOptions.test"
import PathBuilder from "./PathBuilder"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import {
  DEFAULT_OPTIONS,
  ResourceListOptions,
  ResourceListOptionsProvider,
  RESOURCE_LIST_OPTIONS_KEY,
} from "./ResourceListOptionsContext"
import SidebarItem from "./SidebarItem"
import SidebarItemView, { DisabledSidebarItemView } from "./SidebarItemView"
import SidebarResources, {
  SidebarDisabledSectionList,
  SidebarDisabledSectionTitle,
  SidebarGroupName,
  SidebarProps,
} from "./SidebarResources"
import SrOnly from "./SrOnly"
import { StarredResourcesContextProvider } from "./StarredResourcesContext"
import StarResourceButton from "./StarResourceButton"
import {
  nResourceView,
  nResourceWithLabelsView,
  oneResource,
  oneTestResource,
  twoResourceView,
} from "./testdata"
import { ResourceStatus, ResourceView } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

const resourceListOptionsAccessor = accessorsForTesting<ResourceListOptions>(
  RESOURCE_LIST_OPTIONS_KEY,
  sessionStorage
)
const starredItemsAccessor = accessorsForTesting<string[]>(
  "pinned-resources",
  localStorage
)

const SidebarResourcesTestWrapper = ({
  items,
  selected,
  flags,
  resourceListOptions,
}: {
  items: SidebarItem[]
  selected?: string
  flags?: { [key in Flag]?: boolean }
  resourceListOptions?: ResourceListOptions
}) => {
  const features = new Features(flags ?? {})
  const listOptions = resourceListOptions ?? DEFAULT_OPTIONS
  return (
    <MemoryRouter>
      <tiltfileKeyContext.Provider value="test">
        <FeaturesProvider value={features}>
          <StarredResourcesContextProvider>
            <ResourceGroupsContextProvider>
              <ResourceListOptionsProvider>
                <SidebarResources
                  items={items}
                  selected={selected ?? ""}
                  resourceView={ResourceView.Log}
                  pathBuilder={pathBuilder}
                  resourceListOptions={listOptions}
                />
              </ResourceListOptionsProvider>
            </ResourceGroupsContextProvider>
          </StarredResourcesContextProvider>
        </FeaturesProvider>
      </tiltfileKeyContext.Provider>
    </MemoryRouter>
  )
}

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
    sessionStorage.clear()
    localStorage.clear()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    sessionStorage.clear()
    localStorage.clear()
  })

  it("adds items to the starred list when items are starred", () => {
    let ls = new LogStore()
    let items = twoResourceView().uiResources.map((r) => new SidebarItem(r, ls))
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <StarredResourcesContextProvider>
            <ResourceListOptionsProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.Log}
                pathBuilder={pathBuilder}
                resourceListOptions={DEFAULT_OPTIONS}
              />
            </ResourceListOptionsProvider>
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
        tags: {
          action: AnalyticsAction.Click,
          newStarState: "true",
          target: "k8s",
        },
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
            <ResourceListOptionsProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.Log}
                pathBuilder={pathBuilder}
                resourceListOptions={DEFAULT_OPTIONS}
              />
            </ResourceListOptionsProvider>
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
        tags: {
          action: AnalyticsAction.Click,
          newStarState: "false",
          target: "k8s",
        },
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
    "loads %p from browser storage",
    (name, options, expectedItems) => {
      resourceListOptionsAccessor.set(options)

      let ls = new LogStore()
      const items = [
        oneResource({ isBuilding: true }),
        oneTestResource("a"),
        oneTestResource("b"),
      ].map((res) => new SidebarItem(res, ls))

      const root = mount(
        <MemoryRouter>
          <tiltfileKeyContext.Provider value="test">
            <ResourceListOptionsProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.OverviewDetail}
                pathBuilder={pathBuilder}
                resourceListOptions={options}
              />
            </ResourceListOptionsProvider>
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
    "saves option %s to browser storage",
    (name, expectedOptions) => {
      let ls = new LogStore()
      const items = [
        oneResource({ isBuilding: true }),
        oneTestResource("a"),
        oneTestResource("b"),
      ].map((res) => new SidebarItem(res, ls))

      const root = mount(
        <MemoryRouter>
          <tiltfileKeyContext.Provider value="test">
            <ResourceListOptionsProvider>
              <SidebarResources
                items={items}
                selected={""}
                resourceView={ResourceView.OverviewDetail}
                pathBuilder={pathBuilder}
                resourceListOptions={DEFAULT_OPTIONS}
              />
            </ResourceListOptionsProvider>
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

  describe("disabled resources", () => {
    let wrapper: ReactWrapper<SidebarProps, typeof SidebarResources>

    function createSidebarItems(n: number, withLabels = false) {
      const logStore = new LogStore()
      const resourceView = withLabels ? nResourceWithLabelsView : nResourceView
      return resourceView(n).uiResources.map(
        (r) => new SidebarItem(r, logStore)
      )
    }

    describe("when feature flag is enabled", () => {
      beforeEach(() => {
        // Create a list of sidebar items with disable resources interspersed
        const items = createSidebarItems(5)
        items[1].runtimeStatus = ResourceStatus.Disabled
        items[3].runtimeStatus = ResourceStatus.Disabled

        wrapper = mount(
          <SidebarResourcesTestWrapper
            items={items}
            flags={{ [Flag.DisableResources]: true }}
          />
        )
      })

      it("displays disabled resources in their own list", () => {
        const disabledItemsList = wrapper.find(SidebarDisabledSectionList)
        const disabledItemsNames = disabledItemsList
          .find(SidebarItemView)
          .map((item) => item.prop("item").name)
        expect(disabledItemsNames).toEqual(["_1", "_3"])
      })

      it("displays disabled resources list title", () => {
        const disabledItemsList = wrapper.find(SidebarDisabledSectionList)
        expect(disabledItemsList.find(SidebarDisabledSectionTitle).length).toBe(
          1
        )
        // The disabled section title should always be present on the DOM if disabled
        // resources are present and it should be visible to users (and NOT using sr-only)
        expect(disabledItemsList.find(SrOnly).length).toBe(0)
      })

      describe("when there is a resource name filter", () => {
        beforeEach(() => {
          // Create a list of sidebar items with disable resources interspersed
          const items = createSidebarItems(11)
          items[1].runtimeStatus = ResourceStatus.Disabled
          items[3].runtimeStatus = ResourceStatus.Disabled
          items[8].runtimeStatus = ResourceStatus.Disabled

          wrapper = mount(
            <SidebarResourcesTestWrapper
              items={items}
              flags={{ [Flag.DisableResources]: true }}
              resourceListOptions={{
                resourceNameFilter: "1",
                alertsOnTop: true,
              }}
            />
          )
        })

        it("displays disabled resources that match the filter", () => {
          // Expect that all matching resources (enabled + disabled) are displayed
          const resourceNameMatches = wrapper
            .find(SidebarItemView)
            .map((item) => item.prop("item").name)
          expect(resourceNameMatches).toEqual(["_10", "_1"])

          // Expect that all disabled resources appear in their own section
          const disabledItemsList = wrapper.find(SidebarDisabledSectionList)
          const disabledItemsNames = disabledItemsList
            .find(SidebarItemView)
            .map((item) => item.prop("item").name)
          expect(disabledItemsNames).toEqual(["_1"])
        })

        it("displays the disabled resources list title with screen-reader-only class", () => {
          const disabledItemsList = wrapper.find(SidebarDisabledSectionList)
          expect(
            disabledItemsList.find(SidebarDisabledSectionTitle).length
          ).toBe(1)
          // The disabled section title should always be present on the DOM if disabled
          // resources are present, but it should only be available to assistive technology
          expect(disabledItemsList.find(SrOnly).length).toBe(1)
        })
      })

      describe("when there are groups and multiple groups have disabled resources", () => {
        it("displays disabled resources within each group", () => {
          const items = createSidebarItems(10, true)
          // Add disabled items in different label groups based on hardcoded data
          items[2].runtimeStatus = ResourceStatus.Disabled
          items[5].runtimeStatus = ResourceStatus.Disabled

          wrapper = mount(
            <SidebarResourcesTestWrapper
              items={items}
              flags={{ [Flag.DisableResources]: true, [Flag.Labels]: true }}
            />
          )

          expect(wrapper.find(SidebarDisabledSectionList).length).toBe(2)
          expect(wrapper.find(SidebarDisabledSectionTitle).length).toBe(2)
        })
      })
    })

    describe("when feature flag is NOT enabled", () => {
      beforeEach(() => {
        // Create a list of sidebar items with disable resources interspersed
        const items = createSidebarItems(3)
        items[1].runtimeStatus = ResourceStatus.Disabled

        wrapper = mount(
          <SidebarResourcesTestWrapper
            items={items}
            flags={{ [Flag.DisableResources]: false }}
          />
        )
      })

      it("does NOT display disabled resources at all", () => {
        expect(wrapper.find(DisabledSidebarItemView).length).toEqual(0)
        expect(wrapper.find(SidebarItemView).length).toEqual(2)
      })

      it("does NOT display disabled resources list title", () => {
        expect(wrapper.find(SidebarDisabledSectionTitle).length).toBe(0)
      })

      describe("when there are groups and an entire group is disabled", () => {
        it("does NOT display the group section", () => {
          const items = createSidebarItems(5, true)
          // Disable the resource that's in the label group with only one resource
          items[3].runtimeStatus = ResourceStatus.Disabled

          wrapper = mount(
            <SidebarResourcesTestWrapper
              items={items}
              flags={{ [Flag.DisableResources]: false, [Flag.Labels]: true }}
            />
          )

          // Test data hardcodes six label groups (+ one for unlabelled items),
          // so expect that only five total label groups show up when one group
          // has only disabled resources
          const labelGroupNames = wrapper
            .find(SidebarGroupName)
            .map((label) => label.text())
          expect(labelGroupNames.length).toBe(5)
          expect(labelGroupNames).not.toContain("very_long_long_long_label")
        })
      })
    })
  })
})
