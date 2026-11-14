import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { MemoryRouter } from "react-router"
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
    <MemoryRouter
      initialEntries={["/"]}
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
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

describe("expand-all button", () => {
  it("sends analytics onclick", () => {
    let view = nResourceWithLabelsView(3)
    const { container } = render(DisplayOptions({ view }))
    userEvent.click(screen.getByTitle("Expand All"))
  })
})

describe("collapse-all button", () => {
  it("sends analytics onclick", () => {
    let view = nResourceWithLabelsView(3)
    const { container } = render(DisplayOptions({ view }))
    userEvent.click(screen.getByTitle("Collapse All"))
  })
})

describe("alerts on top checkbox", () => {
  afterEach(() => sessionStorage.clear())

  it("turns alerts-on-top sorting on and off", () => {
    let view = nResourceWithLabelsView(3)
    render(DisplayOptions({ view }))
    const checkbox = screen.getByRole("checkbox", { name: "Alerts on top" })

    expect(checkbox).not.toBeChecked()
    userEvent.click(checkbox)
    expect(checkbox).toBeChecked()
    userEvent.click(checkbox)
    expect(checkbox).not.toBeChecked()
  })
})
