import {
  render,
  RenderOptions,
  RenderResult,
  screen,
  waitFor,
  within,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { accessorsForTesting, tiltfileKeyContext } from "./BrowserStorage"
import Features, { FeaturesTestProvider, Flag } from "./feature"
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
import SidebarResources from "./SidebarResources"
import { StarredResourcesContextProvider } from "./StarredResourcesContext"
import { nResourceView, nResourceWithLabelsView, oneResource } from "./testdata"
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

function createSidebarItems(n: number, withLabels = false) {
  const logStore = new LogStore()
  const resourceView = withLabels ? nResourceWithLabelsView : nResourceView
  const resources = resourceView(n).uiResources
  return resources.map((r) => new SidebarItem(r, logStore))
}

function createSidebarItemsWithAlerts() {
  const logStore = new LogStore()
  return [
    oneResource({ isBuilding: true }),
    oneResource({ name: "a" }),
    oneResource({ name: "b" }),
    oneResource({ name: "c", disabled: true }),
  ].map((res) => new SidebarItem(res, logStore))
}

function customRender(
  componentOptions: {
    items: SidebarItem[]
    selected?: string
    resourceListOptions?: ResourceListOptions
  },
  wrapperOptions?: { disableResourcesEnabled: boolean },
  renderOptions?: RenderOptions
) {
  const features = new Features({
    [Flag.DisableResources]: wrapperOptions?.disableResourcesEnabled ?? true,
    [Flag.Labels]: true,
  })
  const listOptions = componentOptions.resourceListOptions ?? DEFAULT_OPTIONS
  return render(
    <SidebarResources
      items={componentOptions.items}
      selected={componentOptions.selected ?? ""}
      resourceView={ResourceView.Log}
      pathBuilder={pathBuilder}
      resourceListOptions={listOptions}
    />,
    {
      wrapper: ({ children }) => (
        <MemoryRouter>
          <tiltfileKeyContext.Provider value="test">
            <FeaturesTestProvider value={features}>
              <StarredResourcesContextProvider>
                <ResourceGroupsContextProvider>
                  <ResourceListOptionsProvider>
                    {children}
                  </ResourceListOptionsProvider>
                </ResourceGroupsContextProvider>
              </StarredResourcesContextProvider>
            </FeaturesTestProvider>
          </tiltfileKeyContext.Provider>
        </MemoryRouter>
      ),
      ...renderOptions,
    }
  )
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

  describe("starring resources", () => {
    const items = createSidebarItems(2)

    it("adds items to the starred list when items are starred", async () => {
      const itemToStar = items[1].name
      customRender({ items: items })

      userEvent.click(
        screen.getByRole("button", { name: `Star ${itemToStar}` })
      )

      await waitFor(() => {
        expect(starredItemsAccessor.get()).toEqual([itemToStar])
      })

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
    })

    it("removes items from the starred list when items are unstarred", async () => {
      starredItemsAccessor.set(items.map((i) => i.name))
      customRender({ items })

      userEvent.click(
        screen.getByRole("button", { name: `Unstar ${items[1].name}` })
      )

      await waitFor(() => {
        expect(starredItemsAccessor.get()).toEqual([items[0].name])
      })

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
    })
  })

  describe("resource list options", () => {
    const items = createSidebarItemsWithAlerts()

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
      (_name, resourceListOptions, expectedItems) => {
        resourceListOptionsAccessor.set(resourceListOptions)

        customRender({ items, resourceListOptions })

        // Find the sidebar items for the expected list
        expectedItems.forEach((item) => {
          expect(screen.getByText(item, { exact: true })).toBeInTheDocument()
        })

        // Check that each option reflects the storage value
        const aotToggle = screen.getByLabelText("Alerts on top")
        expect((aotToggle as HTMLInputElement).checked).toBe(
          resourceListOptions.alertsOnTop
        )

        const resourceNameFilter = screen.getByPlaceholderText(
          "Filter resources by name"
        )
        expect(resourceNameFilter).toHaveValue(
          resourceListOptions.resourceNameFilter
        )

        const disabledToggle = screen.getByLabelText("Show disabled resources")
        expect(disabledToggle).toBeTruthy()
        expect((disabledToggle as HTMLInputElement).checked).toBe(
          resourceListOptions.showDisabledResources
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
        customRender({ items })

        const aotToggle = screen.getByLabelText("Alerts on top")
        if (
          (aotToggle as HTMLInputElement).checked !==
          expectedOptions.alertsOnTop
        ) {
          userEvent.click(aotToggle)
        }

        const resourceNameFilter = screen.getByPlaceholderText(
          "Filter resources by name"
        )
        if (expectedOptions.resourceNameFilter) {
          userEvent.type(resourceNameFilter, expectedOptions.resourceNameFilter)
        }

        const disabledToggle = screen.getByLabelText("Show disabled resources")
        if (
          (disabledToggle as HTMLInputElement).checked !==
          expectedOptions.showDisabledResources
        ) {
          userEvent.click(disabledToggle)
        }

        const observedOptions = resourceListOptionsAccessor.get()
        expect(observedOptions).toEqual(expectedOptions)
      }
    )
  })

  describe("disabled resources", () => {
    describe("when feature flag is enabled and `showDisabledResources` option is true", () => {
      let rerender: RenderResult["rerender"]

      beforeEach(() => {
        // Create a list of sidebar items with disable resources interspersed
        const items = createSidebarItems(5)
        items[1].runtimeStatus = ResourceStatus.Disabled
        items[3].runtimeStatus = ResourceStatus.Disabled

        rerender = customRender({
          items,
          resourceListOptions: {
            ...DEFAULT_OPTIONS,
            showDisabledResources: true,
          },
        }).rerender
      })

      it("displays disabled resources list title", () => {
        expect(
          screen.getByText("Disabled", { exact: true })
        ).toBeInTheDocument()
      })

      it("displays disabled resources in their own list", () => {
        // Get the disabled resources list and query within it
        const disabledResourceList = screen.getByLabelText("Disabled resources")

        expect(within(disabledResourceList).getByText("_1")).toBeInTheDocument()
        expect(within(disabledResourceList).getByText("_3")).toBeInTheDocument()
      })

      describe("when there is a resource name filter", () => {
        beforeEach(() => {
          // Create a list of sidebar items with disable resources interspersed
          const itemsWithFilter = createSidebarItems(11)
          itemsWithFilter[1].runtimeStatus = ResourceStatus.Disabled
          itemsWithFilter[3].runtimeStatus = ResourceStatus.Disabled
          itemsWithFilter[8].runtimeStatus = ResourceStatus.Disabled

          const options = {
            resourceNameFilter: "1",
            alertsOnTop: true,
            showDisabledResources: true,
          }

          rerender(
            <SidebarResources
              items={itemsWithFilter}
              selected=""
              resourceView={ResourceView.Log}
              pathBuilder={pathBuilder}
              resourceListOptions={options}
            />
          )
        })

        it("displays disabled resources that match the filter", () => {
          // Expect that all matching resources (enabled + disabled) are displayed
          expect(screen.getByText("_1", { exact: true })).toBeInTheDocument()
          expect(screen.getByText("_10", { exact: true })).toBeInTheDocument()

          // Expect that all disabled resources appear in their own section
          const disabledItemsList = screen.getByLabelText("Disabled resources")
          expect(within(disabledItemsList).getByText("_1")).toBeInTheDocument()
        })
      })

      describe("when there are groups and multiple groups have disabled resources", () => {
        it("displays disabled resources within each group", () => {
          const itemsWithLabels = createSidebarItems(10, true)
          // Add disabled items in different label groups based on hardcoded data
          itemsWithLabels[2].runtimeStatus = ResourceStatus.Disabled
          itemsWithLabels[5].runtimeStatus = ResourceStatus.Disabled

          rerender(
            <SidebarResources
              items={itemsWithLabels}
              selected=""
              resourceView={ResourceView.Log}
              pathBuilder={pathBuilder}
              resourceListOptions={{
                ...DEFAULT_OPTIONS,
                showDisabledResources: true,
              }}
            />
          )

          expect(screen.getAllByLabelText("Disabled resources")).toHaveLength(2)
        })
      })
    })

    describe("when feature flag is NOT enabled", () => {
      beforeEach(() => {
        // Create a list of sidebar items with disable resources interspersed
        const items = createSidebarItems(5, true)
        // Disable the resource that's in the label group with only one resource
        items[3].runtimeStatus = ResourceStatus.Disabled

        customRender({ items }, { disableResourcesEnabled: false })
      })

      it("does NOT display disabled resources at all", () => {
        expect(screen.queryByLabelText("Disabled resources")).toBeNull()
      })

      it("does NOT display disabled resources list title", () => {
        expect(screen.queryByText("Disabled")).toBeNull()
      })

      describe("when there are groups and an entire group is disabled", () => {
        it("does NOT display the group section", () => {
          // The test data has one group with only disabled resources,
          // so expect that it doesn't show up
          expect(screen.queryByText("very_long_long_long_label")).toBeNull()
        })
      })
    })

    describe("when feature flag is enabled and `showDisabledResources` is false", () => {
      it("does NOT display disabled resources at all", () => {
        expect(screen.queryByLabelText("Disabled resources")).toBeNull()
        expect(screen.queryByText("_1", { exact: true })).toBeNull()
        expect(screen.queryByText("_3", { exact: true })).toBeNull()
      })

      it("does NOT display disabled resources list title", () => {
        expect(screen.queryByText("Disabled", { exact: true })).toBeNull()
      })

      describe("when there are groups and an entire group is disabled", () => {
        it("does NOT display the group section", () => {
          const items = createSidebarItems(5, true)
          // Disable the resource that's in the label group with only one resource
          items[3].runtimeStatus = ResourceStatus.Disabled

          customRender({ items }, { disableResourcesEnabled: false })

          // The test data has one group with only disabled resources,
          // so expect that it doesn't show up
          expect(screen.queryByText("very_long_long_long_label")).toBeNull()
        })
      })
    })
  })
})
