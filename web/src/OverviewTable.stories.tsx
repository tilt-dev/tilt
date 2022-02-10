import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesValueProvider, Flag } from "./feature"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewTable from "./OverviewTable"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import { ResourceListOptionsProvider } from "./ResourceListOptionsContext"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"
import { TiltSnackbarProvider } from "./Snackbar"
import {
  nButtonView,
  nResourceView,
  nResourceWithLabelsView,
  oneResource,
  tiltfileResource,
  twoResourceView,
} from "./testdata"
import { LogLevel } from "./types"

export default {
  title: "New UI/Overview/OverviewTable",
  decorators: [
    (Story: any, context: any) => {
      const features = new Features({
        [Flag.Labels]: context?.args?.labelsEnabled ?? true,
        [Flag.DisableResources]: context?.args?.disableResourcesEnabled ?? true,
      })
      return (
        <MemoryRouter initialEntries={["/"]}>
          <TiltSnackbarProvider>
            <FeaturesValueProvider value={features}>
              <ResourceGroupsContextProvider>
                <ResourceListOptionsProvider>
                  <ResourceSelectionProvider>
                    <div style={{ margin: "-1rem" }}>
                      <Story />
                    </div>
                  </ResourceSelectionProvider>
                </ResourceListOptionsProvider>
              </ResourceGroupsContextProvider>
            </FeaturesValueProvider>
          </TiltSnackbarProvider>
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

export const TwoResources = () => <OverviewTable view={twoResourceView()} />

export const TiltfileWarning = () => {
  let view = nResourceView(10)
  let res = tiltfileResource()

  let logStore = new LogStore()
  let spanId = res!.status!.buildHistory![0].spanID!
  logStore.append({
    spans: {
      [spanId]: { manifestName: res!.metadata!.name },
    },
    segments: [
      { spanId, level: LogLevel.WARN, anchor: true, text: "warning 1!\n" },
      { spanId, level: LogLevel.WARN, anchor: true, text: "warning 2!\n" },
    ],
    fromCheckpoint: 0,
    toCheckpoint: 2,
  })

  view.uiResources[0] = res
  return (
    <LogStoreProvider value={logStore}>
      <OverviewTable view={view} />
    </LogStoreProvider>
  )
}

export const TenResources = () => {
  const view = nResourceView(8)

  // Add a couple disabled resources
  const disableResource9 = oneResource({ disabled: true, name: "_8" })
  const disableResource10 = oneResource({ disabled: true, name: "_9" })
  view.uiResources.push(disableResource9)
  view.uiResources.push(disableResource10)

  return <OverviewTable view={view} />
}

export const TenResourceWithLabels = () => {
  const view = nResourceWithLabelsView(8)

  // Add a couple disabled resources
  const disableResource9 = oneResource({
    disabled: true,
    name: "_8",
    labels: 2,
  })
  const disableResource10 = oneResource({ disabled: true, name: "_9" })
  view.uiResources.push(disableResource9)
  view.uiResources.push(disableResource10)

  return <OverviewTable view={view} />
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
