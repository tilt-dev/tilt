import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsAction, AnalyticsType } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import { OverviewTableDisplayOptions } from "./OverviewTableDisplayOptions"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import {
  ResourceListOptions,
  ResourceListOptionsProvider,
} from "./ResourceListOptionsContext"
import { nResourceWithLabelsView, TestDataView } from "./testdata"

// Helpers
const DisplayOptions = ({
  view,
  resourceListOptions,
}: {
  view: TestDataView
  resourceListOptions?: ResourceListOptions
}) => {
  const features = new Features({
    [Flag.Labels]: true,
  })
  return (
    <MemoryRouter initialEntries={["/"]}>
      <FeaturesTestProvider value={features}>
        <ResourceGroupsContextProvider>
          <ResourceListOptionsProvider
            initialValuesForTesting={resourceListOptions}
          >
            <OverviewTableDisplayOptions resources={view.uiResources} />
          </ResourceListOptionsProvider>
        </ResourceGroupsContextProvider>
      </FeaturesTestProvider>
    </MemoryRouter>
  )
}

beforeEach(() => {
  mockAnalyticsCalls()
})

afterEach(() => {
  cleanupMockAnalyticsCalls()
})

describe("expand-all button", () => {
  it("sends analytics onclick", () => {
    let view = nResourceWithLabelsView(3)
    const { container } = render(DisplayOptions({ view }))
    userEvent.click(screen.getByTitle("Expand All"))
    expectIncrs({
      name: "ui.web.expandAllGroups",
      tags: { action: AnalyticsAction.Click, type: AnalyticsType.Grid },
    })
  })
})

describe("collapse-all button", () => {
  it("sends analytics onclick", () => {
    let view = nResourceWithLabelsView(3)
    const { container } = render(DisplayOptions({ view }))
    userEvent.click(screen.getByTitle("Collapse All"))
    expectIncrs({
      name: "ui.web.collapseAllGroups",
      tags: { action: AnalyticsAction.Click, type: AnalyticsType.Grid },
    })
  })
})
