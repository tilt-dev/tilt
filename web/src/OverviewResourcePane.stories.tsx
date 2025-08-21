import { StylesProvider } from "@material-ui/core/styles"
import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourcePane from "./OverviewResourcePane"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import { ResourceListOptionsProvider } from "./ResourceListOptionsContext"
import { ResourceNavProvider } from "./ResourceNav"
import { SidebarContextProvider } from "./SidebarContext"
import { TiltSnackbarProvider } from "./Snackbar"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import {
  nButtonView,
  nResourceView,
  nResourceWithLabelsView,
  tenResourceView,
  twoResourceView,
} from "./testdata"

export default {
  title: "New UI/OverviewResourcePane",
  decorators: [
    (Story: any, context: any) => {
      const features = new Features({
        [Flag.Labels]: context?.args?.labelsEnabled ?? true,
      })
      return (
        <TiltSnackbarProvider>
          <FeaturesTestProvider value={features}>
            <ResourceGroupsContextProvider>
              <ResourceListOptionsProvider>
                <StarredResourceMemoryProvider>
                  <div style={{ margin: "-1rem", height: "80vh" }}>
                    <StylesProvider injectFirst>
                      <Story />
                    </StylesProvider>
                  </div>
                </StarredResourceMemoryProvider>
              </ResourceListOptionsProvider>
            </ResourceGroupsContextProvider>
          </FeaturesTestProvider>
        </TiltSnackbarProvider>
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

function OverviewResourcePaneHarness(props: {
  name: string
  view: Proto.webviewView
  sidebarClosed?: boolean
}) {
  let { name, view } = props
  let entry = name ? `/r/${props.name}/overview` : `/overview`
  let resources = view?.uiResources || []
  let validateResource = (name: string) =>
    resources.some((res) => res.metadata?.name == name)
  return (
    <MemoryRouter initialEntries={[entry]}>
      <ResourceNavProvider validateResource={validateResource}>
        <SidebarContextProvider sidebarClosedForTesting={props.sidebarClosed}>
          <OverviewResourcePane view={view} isSocketConnected={true} />
        </SidebarContextProvider>
      </ResourceNavProvider>
    </MemoryRouter>
  )
}

export const TwoResources = () => (
  <OverviewResourcePaneHarness name={"vigoda"} view={twoResourceView()} />
)

export const TenResources = () => (
  <OverviewResourcePaneHarness name="vigoda_1" view={tenResourceView()} />
)

export const TenResourcesSidebarCollapsed = () => (
  <OverviewResourcePaneHarness
    name="vigoda_1"
    view={tenResourceView()}
    sidebarClosed={true}
  />
)

export const TwoStarredResources = () => (
  <StarredResourceMemoryProvider initialValueForTesting={["_1", "_2"]}>
    <OverviewResourcePaneHarness name="vigoda_1" view={tenResourceView()} />
  </StarredResourceMemoryProvider>
)

export const TenResourcesLongNames = () => {
  let view = tenResourceView()
  view.uiResources.forEach((r, n) => {
    r.metadata!.name = "elastic-beanstalk-search-stream-" + n
  })
  return <OverviewResourcePaneHarness name="vigoda_1" view={view} />
}

export const TenResourcesWithLabels = () => (
  <OverviewResourcePaneHarness
    name="vigoda_1"
    view={nResourceWithLabelsView(10)}
  />
)

export const TwoButtons = () => {
  const view = nButtonView(2)
  return <OverviewResourcePaneHarness name="vigoda" view={view} />
}

export const TwoButtonsWithEndpoint = () => {
  const view = nButtonView(2)
  view.uiResources[0].status!.endpointLinks = [{ name: "endpoint", url: "foo" }]
  return <OverviewResourcePaneHarness name="vigoda" view={view} />
}

export const TwoButtonsWithPodID = () => {
  const view = nButtonView(2)
  view.uiResources[0].status!.k8sResourceInfo = { podName: "abcdefg" }
  return <OverviewResourcePaneHarness name="vigoda" view={view} />
}

export const TwoButtonsWithEndpointAndPodID = () => {
  const view = nButtonView(2)
  view.uiResources[0].status!.k8sResourceInfo = { podName: "abcdefg" }
  view.uiResources[0].status!.endpointLinks = [{ name: "endpoint", url: "foo" }]
  return <OverviewResourcePaneHarness name="vigoda" view={view} />
}

export const TenButtons = () => (
  <OverviewResourcePaneHarness name="vigoda" view={nButtonView(10)} />
)

export const DisabledButton = () => {
  const view = nButtonView(1)
  view.uiButtons[0].spec!.disabled = true
  return <OverviewResourcePaneHarness name="vigoda" view={view} />
}

export const FullResourceBar = () => {
  let view = tenResourceView()
  let res = view.uiResources[1]
  res.status = res.status || {}
  res.status.endpointLinks = [
    { url: "http://localhost:4001" },
    { url: "http://localhost:4002" },
    { url: "http://localhost:4003" },
  ]
  res.status.k8sResourceInfo = { podName: "my-pod-deadbeef" }
  return <OverviewResourcePaneHarness name="vigoda_1" view={view} />
}

export const TenResourcesWithLogStore = () => {
  let logStore = new LogStore()
  let segments = []
  for (let i = 0; i < 100; i++) {
    segments.push({
      spanId: "build:1",
      text: `Vigoda build line ${i}\n`,
      time: new Date().toString(),
    })
  }
  logStore.append({
    spans: {
      "build:1": { manifestName: "vigoda_1" },
    },
    segments: segments,
  })

  return (
    <LogStoreProvider value={logStore}>
      <OverviewResourcePaneHarness name="vigoda_1" view={tenResourceView()} />
    </LogStoreProvider>
  )
}

export const TenResourcesWithLongLogLines = () => {
  let logStore = new LogStore()
  let segments = []
  for (let i = 0; i < 100; i++) {
    let text = []
    for (let j = 0; j < 10; j++) {
      text.push(`Vigoda build line ${i}`)
    }

    segments.push({
      spanId: "build:1",
      text: text.join(", ") + "\n",
      time: new Date().toString(),
    })
  }
  logStore.append({
    spans: {
      "build:1": { manifestName: "vigoda_1" },
    },
    segments: segments,
  })

  return (
    <LogStoreProvider value={logStore}>
      <OverviewResourcePaneHarness name={"vigoda_1"} view={tenResourceView()} />
    </LogStoreProvider>
  )
}

export const TenResourcesWithBuildLogAndPodLog = () => {
  let logStore = new LogStore()
  let segments = []
  for (let i = 0; i < 10; i++) {
    segments.push({
      spanId: "build:1",
      text: `Vigoda build line ${i}\n`,
      time: new Date().toString(),
    })
  }
  for (let i = 0; i < 10; i++) {
    segments.push({
      spanId: "pod:1",
      text: `Vigoda pod line ${i}\n`,
      time: new Date().toString(),
    })
  }
  logStore.append({
    spans: {
      "build:1": { manifestName: "vigoda_1" },
      "pod:1": { manifestName: "vigoda_1" },
    },
    segments: segments,
  })

  return (
    <LogStoreProvider value={logStore}>
      <OverviewResourcePaneHarness name="vigoda_1" view={tenResourceView()} />
    </LogStoreProvider>
  )
}

export const OneHundredResources = () => (
  <OverviewResourcePaneHarness name="vigoda_1" view={nResourceView(100)} />
)

export const NotFound = () => (
  <OverviewResourcePaneHarness name="does-not-exist" view={twoResourceView()} />
)
