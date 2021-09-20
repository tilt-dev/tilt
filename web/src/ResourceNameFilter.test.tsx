import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import {
  DEFAULT_OPTIONS,
  ResourceListOptions,
  ResourceListOptionsProvider,
  RESOURCE_LIST_OPTIONS_KEY,
} from "./ResourceListOptionsContext"
import {
  ClearResourceNameFilterButton,
  ResourceNameFilter,
  ResourceNameFilterTextField,
} from "./ResourceNameFilter"

const resourceListOptionsAccessor = accessorsForTesting<ResourceListOptions>(
  RESOURCE_LIST_OPTIONS_KEY
)

const ResourceNameFilterTestWrapper = () => (
  <MemoryRouter>
    <tiltfileKeyContext.Provider value="test">
      <ResourceListOptionsProvider>
        <ResourceNameFilter />
      </ResourceListOptionsProvider>
    </tiltfileKeyContext.Provider>
  </MemoryRouter>
)

describe("ResourceNameFilter", () => {
  beforeEach(() => {
    mockAnalyticsCalls()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    localStorage.clear()
  })

  it("displays 'clear' button when there is input", () => {
    resourceListOptionsAccessor.set({
      ...DEFAULT_OPTIONS,
      resourceNameFilter: "wow",
    })
    const root = mount(<ResourceNameFilterTestWrapper />)
    const button = root.find(ClearResourceNameFilterButton)
    expect(button.length).toBe(1)
  })

  it("does NOT display 'clear' button when there is NO input", () => {
    const root = mount(<ResourceNameFilterTestWrapper />)
    const button = root.find(ClearResourceNameFilterButton)
    expect(button.length).toBe(0)
  })

  it("reports analytics when input is cleared", () => {
    resourceListOptionsAccessor.set({
      ...DEFAULT_OPTIONS,
      resourceNameFilter: "wow again",
    })
    const root = mount(<ResourceNameFilterTestWrapper />)
    const button = root.find(ClearResourceNameFilterButton)

    button.simulate("click")

    expectIncrs({
      name: "ui.web.clearResourceNameFilter",
      tags: { action: AnalyticsAction.Click },
    })
  })

  describe("persistent state", () => {
    it("reflects existing value in ResourceListOptionsContext", () => {
      resourceListOptionsAccessor.set({
        ...DEFAULT_OPTIONS,
        resourceNameFilter: "cool resource",
      })
      const root = mount(<ResourceNameFilterTestWrapper />)
      const textField = root.find(ResourceNameFilterTextField)

      expect(textField.prop("value")).toBe("cool resource")
    })

    it("saves input to ResourceListOptionsContext", () => {
      const root = mount(<ResourceNameFilterTestWrapper />)
      const textField = root.find("input")

      textField.simulate("change", { target: { value: "very cool resource" } })

      expect(resourceListOptionsAccessor.get()?.resourceNameFilter).toBe(
        "very cool resource"
      )
    })
  })
})
