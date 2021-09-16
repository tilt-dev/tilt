import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesProvider, Flag } from "./feature"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewTable from "./OverviewTable"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import { TiltSnackbarProvider } from "./Snackbar"
import {
  nButtonView,
  nResourceView,
  nResourceWithLabelsView,
  tenResourceView,
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
      })
      return (
        <MemoryRouter initialEntries={["/"]}>
          <TiltSnackbarProvider>
            <FeaturesProvider value={features}>
              <ResourceGroupsContextProvider>
                <div style={{ margin: "-1rem" }}>
                  <Story />
                </div>
              </ResourceGroupsContextProvider>
            </FeaturesProvider>
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
