import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceBar from "./OverviewResourceBar"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "OverviewResourceBar",
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
