import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import { OverviewTableBulkActions } from "./OverviewTableBulkActions"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"
import { disableButton } from "./testdata"

export default {
  title: "New UI/Overview/OverviewTableBulkActions",
  decorators: [
    (Story: any) => {
      const features = new Features({
        [Flag.Labels]: true,
      })
      return (
        <MemoryRouter initialEntries={["/"]}>
          <FeaturesTestProvider value={features}>
            <ResourceSelectionProvider initialValuesForTesting={["fe", "api"]}>
              <Story />
            </ResourceSelectionProvider>
          </FeaturesTestProvider>
        </MemoryRouter>
      )
    },
  ],
}

export const BulkActionsAllEnabled = () => {
  const a = disableButton("fe", true)
  const b = disableButton("api", true)
  return <OverviewTableBulkActions uiButtons={[a, b]} />
}

export const BulkActionsAllDisabled = () => {
  const a = disableButton("fe", false)
  const b = disableButton("api", false)
  return <OverviewTableBulkActions uiButtons={[a, b]} />
}

export const BulkActionsPartial = () => {
  const a = disableButton("fe", false)
  const b = disableButton("api", true)
  return <OverviewTableBulkActions uiButtons={[a, b]} />
}
