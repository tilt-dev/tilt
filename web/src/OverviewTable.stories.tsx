import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesProvider, Flag } from "./feature"
import OverviewTable from "./OverviewTable"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import {
  nButtonView,
  nResourceView,
  nResourceWithLabelsView,
  tenResourceView,
  twoResourceView,
} from "./testdata"

export default {
  title: "New UI/Overview/OverviewTable",
  decorators: [
    (Story: any, context: any) => {
      const features = new Features({
        [Flag.Labels]: context?.args?.labelsEnabled ?? true,
      })
      return (
        <MemoryRouter initialEntries={["/"]}>
          <FeaturesProvider value={features}>
            <ResourceGroupsContextProvider>
              <div style={{ margin: "-1rem" }}>
                <Story />
              </div>
            </ResourceGroupsContextProvider>
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

export const TwoResources = () => <OverviewTable view={twoResourceView()} />

export const TenResources = () => {
  return <OverviewTable view={tenResourceView()} />
}

export const TenResourceWithLabels = () => {
  return <OverviewTable view={nResourceWithLabelsView(10)} />
}

export const OneHundredResources = () => {
  return <OverviewTable view={nResourceView(100)} />
}

export const OneButton = () => {
  return <OverviewTable view={nButtonView(1)} />
}

export const TenButtons = () => {
  return <OverviewTable view={nButtonView(10)} />
}
