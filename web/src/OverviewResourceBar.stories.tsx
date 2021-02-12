import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceBar from "./OverviewResourceBar"
import {
  nResourceView,
  oneResourceTest,
  tenResourceView,
  twoResourceView,
} from "./testdata"
import { UpdateStatus } from "./types"

type Resource = Proto.webviewResource

export default {
  title: "New UI/Shared/OverviewResourceBar",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ margin: "-1rem" }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
}

export const TwoResources = () => (
  <OverviewResourceBar view={twoResourceView()} />
)

export const TenResources = () => (
  <OverviewResourceBar view={tenResourceView()} />
)

export const TenResourcesErrorsAndWarnings = () => {
  let view = tenResourceView() as any
  view.resources[0].updateStatus = UpdateStatus.Error
  view.resources[1].buildHistory[0].warnings = ["warning time"]
  view.resources[5].updateStatus = UpdateStatus.Error
  return <OverviewResourceBar view={view} />
}

export const OneHundredResources = () => (
  <OverviewResourceBar view={nResourceView(100)} />
)

export const UpgradeAvailable = () => {
  let view = twoResourceView()
  view.suggestedTiltVersion = "0.18.1"
  view.runningTiltBuild = { version: "0.18.0", dev: false }
  view.versionSettings = { checkUpdates: true }
  return <OverviewResourceBar view={view} />
}

export const WithTests = () => {
  let view = twoResourceView()
  view.resources.push(oneResourceTest(), oneResourceTest())
  return <OverviewResourceBar view={view} />
}
