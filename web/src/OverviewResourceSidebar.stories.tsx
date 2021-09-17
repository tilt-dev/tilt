import React from "react"
import { MemoryRouter } from "react-router"
import SplitPane from "react-split-pane"
import Features, { FeaturesProvider, Flag } from "./feature"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import { ResourceListOptionsProvider } from "./ResourceListOptionsContext"
import { Width } from "./style-helpers"
import {
  nResourceView,
  nResourceWithLabelsView,
  oneResource,
  oneResourceNoAlerts,
  oneResourceTest,
  tenResourceView,
  tiltfileResource,
  twoResourceView,
} from "./testdata"
import { LogLevel, UIResource, UpdateStatus } from "./types"

export default {
  title: "New UI/Log View/OverviewResourceSidebar",
  decorators: [
    (Story: any, context: any) => {
      const features = new Features({
        [Flag.Labels]: context?.args?.labelsEnabled ?? true,
      })
      return (
        <MemoryRouter initialEntries={["/"]}>
          <FeaturesProvider value={features}>
            <ResourceGroupsContextProvider>
              <div style={{ margin: "-1rem", height: "80vh" }}>
                <SplitPane
                  split="vertical"
                  minSize={Width.sidebarDefault}
                  defaultSize={Width.sidebarDefault}
                >
                  <Story />
                </SplitPane>
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

export const TwoResources = () => (
  <OverviewResourceSidebar name={"vigoda"} view={twoResourceView()} />
)

export const TenResources = () => (
  <OverviewResourceSidebar name={"vigoda_1"} view={tenResourceView()} />
)

export const TenResourcesWithLabels = () => (
  <OverviewResourceSidebar
    name={"vigoda_1"}
    view={nResourceWithLabelsView(10)}
  />
)

export const OneHundredResources = () => (
  <OverviewResourceSidebar name={"vigoda_1"} view={nResourceView(100)} />
)

export function TwoResourcesTwoTests() {
  let all: UIResource[] = [
    tiltfileResource(),
    oneResource(),
    oneResourceNoAlerts(),
    oneResourceTest(),
    oneResourceTest(),
  ]
  all[2].metadata = { name: "snack" }
  all[3].metadata = { name: "beep" }
  let view = { uiResources: all, tiltfileKey: "test" }
  return <OverviewResourceSidebar name={""} view={view} />
}

export function TestsWithErrors() {
  let logStore = new LogStore()
  let segments = []
  let spans = {} as any
  let all: UIResource[] = [tiltfileResource()]
  for (let i = 0; i < 8; i++) {
    let test = oneResourceTest()
    let name = "test_" + i
    test.metadata = { name: name }

    let spanId = "build-" + i
    spans[spanId] = { manifestName: name }

    test.status!.buildHistory![0].spanID = spanId
    if (i % 2 === 0) {
      test.status!.buildHistory![0].error = "egads!"
      test.status!.updateStatus = UpdateStatus.Error

      segments.push({
        spanId,
        text: `egads ${i}!\n`,
        time: new Date().toString(),
        level: LogLevel.ERROR,
        anchor: true,
      })
    }
    all.push(test)
  }

  logStore.append({ spans, segments })

  let view = { uiResources: all, tiltfileKey: "test" }
  return (
    <LogStoreProvider value={logStore}>
      <ResourceListOptionsProvider>
        <OverviewResourceSidebar name={""} view={view} />
      </ResourceListOptionsProvider>
    </LogStoreProvider>
  )
}
