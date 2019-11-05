import { Alert } from "./alerts"
import { Facet } from "./facets"

export enum SocketState {
  Loading,
  Reconnecting,
  Closed,
  Active,
}

export enum ResourceView {
  Log,
  Alerts,
  Facets = 2,
}

export enum TriggerMode {
  TriggerModeAuto,
  TriggerModeManual,
}

export type Build = {
  error: {} | string | null
  startTime: string
  log: string
  finishTime: string
  edits: Array<string> | null
  isCrashRebuild: boolean
  warnings: Array<string> | null
}

export type TiltBuild = {
  version: string
  date: string
  dev: boolean
}

// what is the status of the resource in the cluster
export enum RuntimeStatus {
  Ok = "ok",
  Pending = "pending",
  Error = "error",
  Unknown = "unknown",
}

// What is the status of the resource with respect to Tilt
export enum ResourceStatus {
  BuildQueued, // in auto, if you have changed a file but an affected build hasn't started yet. In manual after you have clicked build, before it has started building
  Building,
  Error,
  Warning,
  Deploying,
  Deployed, // defer to RuntimeStatus
}

export type Resource = {
  name: string
  combinedLog: string
  buildHistory: Array<any>
  crashLog: string
  currentBuild: any
  directoriesWatched: Array<any>
  endpoints: Array<string>
  podID: string
  isTiltfile: boolean
  lastDeployTime: string
  pathsWatched: Array<string>
  pendingBuildEdits: Array<string>
  pendingBuildReason: number
  pendingBuildSince: string
  k8sResourceInfo?: K8sResourceInfo
  dcResourceInfo?: DCResourceInfo
  runtimeStatus: string
  triggerMode: TriggerMode
  hasPendingChanges: boolean
  alerts: Array<Alert>
  facets: Array<Facet>
  queued: boolean
}
export type K8sResourceInfo = {
  podName: string
  podCreationTime: string
  podUpdateStartTime: string
  podStatus: string
  podStatusMessage: string
  podRestarts: number
  podLog: string
}
export type DCResourceInfo = {
  configPaths: Array<string>
  containerStatus: string
  containerID: string
  log: string
  startTime: string
}

export type SnapshotHighlight = {
  beginningLogID: string
  endingLogID: string
  text: string
}

export enum ShowFatalErrorModal {
  Default,
  Show,
  Hide,
}

export type WebView = {
  resources: Array<Resource>
  log: string
  logTimestamps: boolean
  needsAnalyticsNudge: boolean
  runningTiltBuild: TiltBuild
  latestTiltBuild: TiltBuild
  featureFlags: { [featureFlag: string]: boolean }
  tiltCloudUsername: string
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string
  fatalError: string | undefined
}

export type Snapshot = {
  view: WebView
  isSidebarClosed: boolean
  path?: string
  snapshotHighlight?: SnapshotHighlight | null
}

export type HudState = {
  view: WebView
  isSidebarClosed: boolean
  snapshotLink: string
  showSnapshotModal: boolean
  showFatalErrorModal: ShowFatalErrorModal
  snapshotHighlight: SnapshotHighlight | undefined
  socketState: SocketState
}
