import React from "react"
import { MemoryRouter } from "react-router"
import HeaderBar from "./HeaderBar"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"
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
  view.uiResources[0].status.updateStatus = UpdateStatus.Error
  view.uiResources[1].status.buildHistory[0].warnings = ["warning time"]
  view.uiResources[5].status.updateStatus = UpdateStatus.Error
  return <HeaderBar view={view} />
}

export const OneHundredResources = () => <HeaderBar view={nResourceView(100)} />

export const UpgradeAvailable = () => {
  let view = twoResourceView()
  let status = view.uiSession!.status
  status!.suggestedTiltVersion = "0.18.1"
  status!.runningTiltBuild = { version: "0.18.0", dev: false }
  status!.versionSettings = { checkUpdates: true }
  return <HeaderBar view={view} />
}
