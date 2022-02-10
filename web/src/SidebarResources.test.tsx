import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
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
import Features, { FeaturesValueProvider, Flag } from "./feature"
import LogStore from "./LogStore"
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
import { StarredResourcesContextProvider } from "./StarredResourcesContext"
import StarResourceButton from "./StarResourceButton"
import {
  nResourceView,
  nResourceWithLabelsView,
  oneResource,
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
  const features = new Features(
    flags ?? { [Flag.DisableResources]: true, [Flag.Labels]: true }
  )
  const listOptions = resourceListOptions ?? DEFAULT_OPTIONS
  return (
    <MemoryRouter>
      <tiltfileKeyContext.Provider value="test">
        <FeaturesValueProvider value={features}>
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
        </FeaturesValueProvider>
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
    [
      "showDisabledResources",
      { ...DEFAULT_OPTIONS, showDisabledResources: true },
      ["vigoda", "a", "b", "c"],
    ],
  ]
  test.each(loadCases)(
    "loads %p from browser storage",
    (_name, options, expectedItems) => {
      resourceListOptionsAccessor.set(options)

      let ls = new LogStore()
      const items = [
        oneResource({ isBuilding: true }),
        oneResource({ name: "a" }),
        oneResource({ name: "b" }),
        oneResource({ name: "c", disabled: true }),
      ].map((res) => new SidebarItem(res, ls))

      render(
        <SidebarResourcesTestWrapper
          items={items}
          resourceListOptions={options}
        />
      )

      // Find the sidebar items for the expected list
      expectedItems.forEach((item) => {
        expect(screen.getByText(item, { exact: true })).toBeTruthy()
      })

      // Check that each option reflects the storage value
      const aotToggle = screen.queryByLabelText("Alerts on top")
      expect(aotToggle).toBeTruthy()
      expect((aotToggle as HTMLInputElement).checked).toBe(options.alertsOnTop)

      const resourceNameFilter = screen.queryByPlaceholderText(
        "Filter resources by name"
      )
      expect(resourceNameFilter).toBeTruthy()
      expect((resourceNameFilter as HTMLInputElement).value).toBe(
        options.resourceNameFilter
      )

      const disabledToggle = screen.queryByLabelText("Show disabled resources")
      expect(disabledToggle).toBeTruthy()
      expect((disabledToggle as HTMLInputElement).checked).toBe(
        options.showDisabledResources
      )
    }
  )

  const saveCases: [string, ResourceListOptions][] = [
    ["alertsOnTop", { ...DEFAULT_OPTIONS, alertsOnTop: true }],
    ["resourceNameFilter", { ...DEFAULT_OPTIONS, resourceNameFilter: "foo" }],
    [
      "showDisabledResources",
      { ...DEFAULT_OPTIONS, showDisabledResources: true },
    ],
  ]
  test.each(saveCases)(
    "saves option %s to browser storage",
    (_name, expectedOptions) => {
      let ls = new LogStore()
      const items = [
        oneResource({ isBuilding: true }),
        oneResource({ name: "a" }),
        oneResource({ name: "b" }),
        oneResource({ name: "c", disabled: true }),
      ].map((res) => new SidebarItem(res, ls))

      render(<SidebarResourcesTestWrapper items={items} />)

      const aotToggle = screen.queryByLabelText("Alerts on top")
      expect(aotToggle).toBeTruthy()
      if (
        (aotToggle as HTMLInputElement).checked !== expectedOptions.alertsOnTop
      ) {
        userEvent.click(aotToggle as HTMLInputElement)
      }

      const resourceNameFilter = screen.queryByPlaceholderText(
        "Filter resources by name"
      )
      expect(resourceNameFilter).toBeTruthy()
      if (expectedOptions.resourceNameFilter) {
        userEvent.type(
          resourceNameFilter as HTMLInputElement,
          expectedOptions.resourceNameFilter
        )
      }

      const disabledToggle = screen.queryByLabelText("Show disabled resources")
      expect(disabledToggle).toBeTruthy()
      if (
        (disabledToggle as HTMLInputElement).checked !==
        expectedOptions.showDisabledResources
      ) {
        userEvent.click(disabledToggle as HTMLInputElement)
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

    describe("when feature flag is enabled and `showDisabledResources` option is true", () => {
      beforeEach(() => {
        // Create a list of sidebar items with disable resources interspersed
        const items = createSidebarItems(5)
        items[1].runtimeStatus = ResourceStatus.Disabled
        items[3].runtimeStatus = ResourceStatus.Disabled

        wrapper = mount(
          <SidebarResourcesTestWrapper
            items={items}
            flags={{ [Flag.DisableResources]: true }}
            resourceListOptions={{
              ...DEFAULT_OPTIONS,
              showDisabledResources: true,
            }}
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
                showDisabledResources: true,
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
              resourceListOptions={{
                ...DEFAULT_OPTIONS,
                showDisabledResources: true,
              }}
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

    describe("when feature flag is enabled and `showDisabledResources` is false", () => {
      beforeEach(() => {
        // Create a list of sidebar items with disable resources interspersed
        const items = createSidebarItems(3)
        items[1].runtimeStatus = ResourceStatus.Disabled

        wrapper = mount(
          <SidebarResourcesTestWrapper
            items={items}
            flags={{ [Flag.DisableResources]: true }}
            resourceListOptions={{
              ...DEFAULT_OPTIONS,
              showDisabledResources: false,
            }}
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
              resourceListOptions={{
                ...DEFAULT_OPTIONS,
                showDisabledResources: false,
              }}
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
