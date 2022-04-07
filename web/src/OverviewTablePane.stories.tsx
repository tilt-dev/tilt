import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import OverviewTablePane from "./OverviewTablePane"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import { ResourceListOptionsProvider } from "./ResourceListOptionsContext"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"
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
        [Flag.DisableResources]: context?.args?.disableResourcesEnabled ?? true,
      })
      return (
        <MemoryRouter initialEntries={["/"]}>
          <FeaturesTestProvider value={features}>
            <ResourceListOptionsProvider>
              <StarredResourceMemoryProvider>
                <ResourceGroupsContextProvider>
                  <ResourceSelectionProvider>
                    <div style={{ margin: "-1rem", height: "80vh" }}>
                      <Story />
                    </div>
                  </ResourceSelectionProvider>
                </ResourceGroupsContextProvider>
              </StarredResourceMemoryProvider>
            </ResourceListOptionsProvider>
          </FeaturesTestProvider>
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
    disableResourcesEnabled: {
      name: "See disabled resources and bulk actions",
      control: {
        type: "boolean",
      },
      defaultValue: true,
    },
  },
}

export const TwoResources = () => (
  <OverviewTablePane view={twoResourceView()} tiltConnected={true} />
)

export const TenResources = () => (
  <OverviewTablePane view={tenResourceView()} tiltConnected={true} />
)

export const TenResourcesWithLabels = () => (
  <OverviewTablePane view={nResourceWithLabelsView(10)} tiltConnected={true} />
)

export const OneHundredResources = () => (
  <OverviewTablePane view={nResourceView(100)} tiltConnected={true} />
)

export const OneHundredResourcesOneStar = () => (
  <StarredResourceMemoryProvider initialValueForTesting={["vigoda_2"]}>
    <OverviewTablePane view={nResourceView(100)} tiltConnected={true} />
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
      <OverviewTablePane view={nResourceView(100)} tiltConnected={true} />
    </StarredResourceMemoryProvider>
  )
}
