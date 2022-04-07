import React from "react"
import { MemoryRouter } from "react-router"
import { AnalyticsType } from "./analytics"
import { GlobalNav } from "./GlobalNav"
import HeaderBar from "./HeaderBar"
import { useSnapshotAction } from "./snapshot"
import {
  clusterConnection,
  nResourceView,
  tenResourceView,
  twoResourceView,
} from "./testdata"
import { UpdateStatus } from "./types"
import { showUpdate } from "./UpdateDialog"

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
  <HeaderBar view={twoResourceView()} currentPage={AnalyticsType.Detail} />
)

export const TenResources = () => (
  <HeaderBar view={tenResourceView()} currentPage={AnalyticsType.Detail} />
)

export const TenResourcesErrorsAndWarnings = () => {
  let view = tenResourceView() as any
  view.uiResources[0].status.updateStatus = UpdateStatus.Error
  view.uiResources[1].status.buildHistory[0].warnings = ["warning time"]
  view.uiResources[5].status.updateStatus = UpdateStatus.Error
  return <HeaderBar view={view} currentPage={AnalyticsType.Grid} />
}

export const OneHundredResources = () => (
  <HeaderBar view={nResourceView(100)} currentPage={AnalyticsType.Grid} />
)

export const UpgradeAvailable = () => {
  let view = twoResourceView()
  let status = view.uiSession!.status
  status!.suggestedTiltVersion = "0.18.1"
  status!.runningTiltBuild = { version: "0.18.0", dev: false }
  status!.versionSettings = { checkUpdates: true }
  return <HeaderBar view={view} currentPage={AnalyticsType.Detail} />
}

// TODO (lizz): Use HeaderBar component instead of GlobalNav
// when design & implementation are finalized
export const NavWithClusterConnectionHealth = () => {
  const view = nResourceView(5)
  const k8sConnection = clusterConnection()
  const session = view.uiSession?.status

  return (
    <GlobalNav
      isSnapshot={false}
      runningBuild={session?.runningTiltBuild}
      snapshot={useSnapshotAction()}
      showUpdate={showUpdate(view)}
      suggestedVersion={session?.suggestedTiltVersion}
      tiltCloudSchemeHost=""
      tiltCloudTeamID=""
      tiltCloudTeamName=""
      tiltCloudUsername=""
      clusterConnections={[k8sConnection]}
    />
  )
}

export const NavWithClusterConnectionError = () => {
  const view = nResourceView(5)
  const k8sConnection = clusterConnection(
    'Get "https://kubernetes.docker.internal:6443/version?timeout=32s": dial tcp 127.0.0.1:6443: connect: connection refused'
  )
  const session = view.uiSession?.status

  return (
    <GlobalNav
      isSnapshot={false}
      runningBuild={session?.runningTiltBuild}
      snapshot={useSnapshotAction()}
      showUpdate={showUpdate(view)}
      suggestedVersion={session?.suggestedTiltVersion}
      tiltCloudSchemeHost=""
      tiltCloudTeamID=""
      tiltCloudTeamName=""
      tiltCloudUsername=""
      clusterConnections={[k8sConnection]}
    />
  )
}
