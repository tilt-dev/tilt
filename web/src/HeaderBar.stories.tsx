import React from "react"
import { MemoryRouter } from "react-router"
import HeaderBar from "./HeaderBar"
import {
  nResourceView,
  oneResourceTest,
  tenResourceView,
  twoResourceView,
} from "./testdata"
import { UpdateStatus } from "./types"

export default {
  title: "New UI/Shared/HeaderBar",
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

export const TwoResources = () => <HeaderBar view={twoResourceView()} />

export const TenResources = () => <HeaderBar view={tenResourceView()} />

export const TenResourcesErrorsAndWarnings = () => {
  let view = tenResourceView() as any
  view.resources[0].updateStatus = UpdateStatus.Error
  view.resources[1].buildHistory[0].warnings = ["warning time"]
  view.resources[5].updateStatus = UpdateStatus.Error
  return <HeaderBar view={view} />
}

export const OneHundredResources = () => <HeaderBar view={nResourceView(100)} />

export const UpgradeAvailable = () => {
  let view = twoResourceView()
  view.suggestedTiltVersion = "0.18.1"
  view.runningTiltBuild = { version: "0.18.0", dev: false }
  view.versionSettings = { checkUpdates: true }
  return <HeaderBar view={view} />
}

export const WithTests = () => {
  let view = twoResourceView()
  view.resources.push(oneResourceTest(), oneResourceTest())
  return <HeaderBar view={view} />
}
