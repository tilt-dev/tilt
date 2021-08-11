import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesProvider, Flag } from "./feature"
import OverviewTablePane from "./OverviewTablePane"
import { ResourceGroupsProvider } from "./ResourceGroupsContext"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import {
  nResourceView,
  nResourceWithLabelsView,
  tenResourceView,
  twoResourceView,
} from "./testdata"

export default {
  title: "New UI/OverviewTablePane",
  decorators: [
    (Story: any, context: any) => {
      const features = new Features({
        [Flag.Labels]: context?.args?.labelsEnabled ?? true,
      })
      return (
        <MemoryRouter initialEntries={["/"]}>
          <FeaturesProvider value={features}>
            <ResourceGroupsProvider>
              <StarredResourceMemoryProvider>
                <div style={{ margin: "-1rem", height: "80vh" }}>
                  <Story />
                </div>
              </StarredResourceMemoryProvider>
            </ResourceGroupsProvider>
          </FeaturesProvider>
        </MemoryRouter>
      )
    },
  ],
  argTypes: {
    labelsEnabled: {
      name: "Group resources by label enabled",
      control: {
        type: "boolean",
      },
      defaultValue: true,
    },
  },
}

export const TwoResources = () => <OverviewTablePane view={twoResourceView()} />

export const TenResources = () => <OverviewTablePane view={tenResourceView()} />

export const TenResourcesWithLabels = () => (
  <OverviewTablePane view={nResourceWithLabelsView(10)} />
)

export const OneHundredResources = () => (
  <OverviewTablePane view={nResourceView(100)} />
)

export const OneHundredResourcesOneStar = () => (
  <StarredResourceMemoryProvider initialValueForTesting={["vigoda_2"]}>
    <OverviewTablePane view={nResourceView(100)} />
  </StarredResourceMemoryProvider>
)

export const OneHundredResourcesTenStars = () => {
  let items = [
    "vigoda_1",
    "vigoda_11",
    "vigoda_21",
    "vigoda_31",
    "vigoda_41",
    "vigoda_51",
    "vigoda_61",
    "vigoda_71",
    "vigoda_81",
    "vigoda_91",
  ]
  return (
    <StarredResourceMemoryProvider initialValueForTesting={items}>
      <OverviewTablePane view={nResourceView(100)} />
    </StarredResourceMemoryProvider>
  )
}
