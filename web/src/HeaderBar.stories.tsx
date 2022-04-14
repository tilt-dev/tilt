import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsType } from "./analytics"
import HeaderBar from "./HeaderBar"
import {
  clusterConnection,
  nResourceView,
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

export const TwoResources = () => (
  <HeaderBar
    view={twoResourceView()}
    currentPage={AnalyticsType.Detail}
    isSocketConnected={true}
  />
)

export const TenResources = () => (
  <HeaderBar
    view={tenResourceView()}
    currentPage={AnalyticsType.Detail}
    isSocketConnected={true}
  />
)

export const TenResourcesErrorsAndWarnings = () => {
  let view = tenResourceView() as any
  view.uiResources[0].status.updateStatus = UpdateStatus.Error
  view.uiResources[1].status.buildHistory[0].warnings = ["warning time"]
  view.uiResources[5].status.updateStatus = UpdateStatus.Error
  return (
    <HeaderBar
      view={view}
      currentPage={AnalyticsType.Grid}
      isSocketConnected={true}
    />
  )
}

export const OneHundredResources = () => (
  <HeaderBar
    view={nResourceView(100)}
    currentPage={AnalyticsType.Grid}
    isSocketConnected={true}
  />
)

export const UpgradeAvailable = () => {
  let view = twoResourceView()
  let status = view.uiSession!.status
  status!.suggestedTiltVersion = "0.18.1"
  status!.runningTiltBuild = { version: "0.18.0", dev: false }
  status!.versionSettings = { checkUpdates: true }
  return (
    <HeaderBar
      view={view}
      currentPage={AnalyticsType.Detail}
      isSocketConnected={true}
    />
  )
}

export const HealthyClusterConnection = () => {
  const view = nResourceView(5)
  const k8sConnection = clusterConnection()
  view.clusters = [k8sConnection]

  return (
    <HeaderBar
      view={view}
      currentPage={AnalyticsType.Detail}
      isSocketConnected={true}
    />
  )
}

export const UnhealthyClusterConnection = () => {
  const view = nResourceView(5)
  const k8sConnection = clusterConnection(
    'Get "https://kubernetes.docker.internal:6443/version?timeout=32s": dial tcp 127.0.0.1:6443: connect: connection refused'
  )
  view.clusters = [k8sConnection]

  return (
    <HeaderBar
      view={view}
      currentPage={AnalyticsType.Detail}
      isSocketConnected={true}
    />
  )
}
