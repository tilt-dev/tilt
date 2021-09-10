import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import {
  GlobalOptions,
  GlobalOptionsContextProvider,
} from "./GlobalOptionsContext"
import { accessorsForTesting, tiltfileKeyContext } from "./LocalStorage"
import {
  ClearResourceNameFilterButton,
  ResourceNameFilter,
  ResourceNameFilterTextField,
} from "./ResourceNameFilter"

const globalOptionsAccessor = accessorsForTesting<GlobalOptions>(
  "global-options"
)

const ResourceNameFilterTestWrapper = () => (
  <MemoryRouter>
    <tiltfileKeyContext.Provider value="test">
      <GlobalOptionsContextProvider>
        {/* <GlobalOptionsContextProvider initialValuesForTesting={{"resourceNameFilter": initialFilter }}> */}
        <ResourceNameFilter />
      </GlobalOptionsContextProvider>
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
    globalOptionsAccessor.set({ resourceNameFilter: "wow" })
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
    globalOptionsAccessor.set({ resourceNameFilter: "wow again" })
    const root = mount(<ResourceNameFilterTestWrapper />)
    const button = root.find(ClearResourceNameFilterButton)

    button.simulate("click")

    expectIncrs({
      name: "ui.web.clearResourceNameFilter",
      tags: { action: AnalyticsAction.Click },
    })
  })

  describe("persistent state", () => {
    it("reflects existing value in GlobalOptionsContext", () => {
      globalOptionsAccessor.set({ resourceNameFilter: "cool resource" })
      const root = mount(<ResourceNameFilterTestWrapper />)
      const textField = root.find(ResourceNameFilterTextField)

      expect(textField.prop("value")).toBe("cool resource")
    })

    // TODO (lizz): figure out how the accessor works with testing and get this passing!
    xit("saves input to GlobalOptionsContext", () => {
      const root = mount(<ResourceNameFilterTestWrapper />)
      const textField = root.find(ResourceNameFilterTextField)

      textField.simulate("change", { target: { value: "very cool resource" } })
      root.update()

      expect(globalOptionsAccessor.get()?.resourceNameFilter).toBe(
        "very cool resource"
      )
    })
  })
})
