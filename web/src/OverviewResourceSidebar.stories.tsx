import React from "react"
import { MemoryRouter } from "react-router"
import SplitPane from "react-split-pane"
import Features, { FeaturesProvider, Flag } from "./feature"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import PathBuilder from "./PathBuilder"
import { Width } from "./style-helpers"
import {
  nResourceView,
  oneResource,
  oneResourceCrashedOnStart,
  oneResourceFailedToBuild,
  oneResourceNoAlerts,
  oneResourceTest,
  tenResourceView,
  tiltfileResource,
  twoResourceView,
} from "./testdata"
import { UpdateStatus } from "./types"

type UIResource = Proto.v1alpha1UIResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

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
            <div style={{ margin: "-1rem", height: "80vh" }}>
              <SplitPane
                split="vertical"
                minSize={Width.sidebarDefault}
                defaultSize={Width.sidebarDefault}
              >
                <Story />
              </SplitPane>
            </div>
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

export const OneHundredResources = () => (
  <OverviewResourceSidebar name={"vigoda_1"} view={nResourceView(100)} />
)

export const ResourcesWithLabels = () => {
  const view = nResourceView(10)
  for (let i = 0; i < 10; i++) {
    const labels: { [key: string]: string } = {}
    // The first item is a Tiltfile, so don't apply a label to it
    if (i > 0 && i < 5) {
      labels["frontend"] = "frontend"
    }
    if (i % 2) {
      labels["test"] = "test"
    }

    if (i === 3) {
      labels["very_long_long_long_label"] = "very_long_long_long_label"
    }

    view.uiResources[i].metadata!.labels = labels
  }

  // Non-happy path resources
  const [failedBuild] = oneResourceFailedToBuild()
  failedBuild.metadata!.labels = {
    test: "test",
    backend: "backend",
  }
  failedBuild.metadata!.name = "resource_11"
  view.uiResources.push(failedBuild)

  const [crashedStart] = oneResourceCrashedOnStart()
  crashedStart.metadata!.labels = {
    javascript: "javascript",
    backend: "frontend",
  }
  crashedStart.metadata!.name = "resource_12"
  view.uiResources.push(crashedStart)

  return <OverviewResourceSidebar name={"vigoda_1"} view={view} />
}

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
  let all: UIResource[] = [tiltfileResource()]
  for (let i = 0; i < 8; i++) {
    let test = oneResourceTest()
    test.metadata = { name: "test_" + i }
    if (i % 2 === 0) {
      test.status!.buildHistory![0].error = "egads!"
      test.status!.updateStatus = UpdateStatus.Error
    }
    all.push(test)
  }
  let view = { uiResources: all, tiltfileKey: "test" }
  return <OverviewResourceSidebar name={""} view={view} />
}
