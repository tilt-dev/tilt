import React, { useEffect, useState } from "react"
import { MemoryRouter } from "react-router"
import SplitPane from "react-split-pane"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import { ResourceListOptionsProvider } from "./ResourceListOptionsContext"
import { SidebarMemoryProvider, useSidebarContext } from "./SidebarContext"
import { Width } from "./style-helpers"
import {
  nResourceView,
  nResourceWithLabelsView,
  oneResource,
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
          <FeaturesTestProvider value={features}>
            <ResourceGroupsContextProvider>
              <SidebarMemoryProvider>
                <div style={{ margin: "-1rem", height: "80vh" }}>
                  <Story />
                </div>
              </SidebarMemoryProvider>
            </ResourceGroupsContextProvider>
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

function OverviewResourceSidebarHarness(props: {
  name: string
  view: Proto.webviewView
}) {
  let { name, view } = props
  const { isSidebarOpen, setSidebarOpen, setSidebarClosed } =
    useSidebarContext()

  const [paneSize, setPaneSize] = useState<number>(
    isSidebarOpen ? Width.sidebarDefault : Width.sidebarMinimum
  )

  useEffect(() => {
    setPaneSize(
      isSidebarOpen ? Width.sidebarDefault : Width.sidebarMinimum + 0.01
    )
  }, [isSidebarOpen])

  const handleSplitPaneResize = (newSize: number) => {
    if (newSize < Width.sidebarBreakpoint && isSidebarOpen) {
      setSidebarClosed()
    } else if (newSize >= Width.sidebarBreakpoint && !isSidebarOpen) {
      setSidebarOpen()
    }
  }
  return (
    <SplitPane
      split="vertical"
      size={paneSize}
      minSize={Width.sidebarMinimum}
      onChange={handleSplitPaneResize}
      onDragFinished={() =>
        setPaneSize(isSidebarOpen ? Width.sidebarDefault : Width.sidebarMinimum)
      }
    >
      <OverviewResourceSidebar name={name} view={view} />
      <div></div>
    </SplitPane>
  )
}

export const TwoResources = () => (
  <OverviewResourceSidebarHarness name={"vigoda"} view={twoResourceView()} />
)

export const TenResources = () => (
  <OverviewResourceSidebarHarness name={"vigoda_1"} view={tenResourceView()} />
)

export const TenResourcesWithLabels = () => (
  <OverviewResourceSidebarHarness
    name={"vigoda_1"}
    view={nResourceWithLabelsView(10)}
  />
)

export const OneHundredResources = () => (
  <OverviewResourceSidebarHarness name={"vigoda_1"} view={nResourceView(100)} />
)

export function TwoResourcesTwoTests() {
  let all: UIResource[] = [
    tiltfileResource(),
    oneResource({ isBuilding: true }),
    oneResource({ name: "snack" }),
    oneResource({ name: "beep" }),
    oneResource({ name: "boop" }),
  ]
  let view = { uiResources: all, tiltfileKey: "test" }
  return <OverviewResourceSidebarHarness name={""} view={view} />
}

export function TestsWithErrors() {
  let logStore = new LogStore()
  let segments = []
  let spans = {} as any
  let all: UIResource[] = [tiltfileResource()]
  for (let i = 0; i < 8; i++) {
    let name = "test_" + i
    let test = oneResource({ name })

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
    <MemoryRouter>
      <LogStoreProvider value={logStore}>
        <ResourceListOptionsProvider>
          <OverviewResourceSidebarHarness name={""} view={view} />
        </ResourceListOptionsProvider>
      </LogStoreProvider>
    </MemoryRouter>
  )
}
