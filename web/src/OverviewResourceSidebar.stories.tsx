import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import PathBuilder from "./PathBuilder"
import {
  nResourceView,
  oneResource,
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
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ margin: "-1rem", height: "80vh" }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
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
