import { Alert } from "./alerts"

export enum ResourceView {
  Log,
  Alerts,
}

export enum TriggerMode {
  TriggerModeAuto,
  TriggerModeManual,
}

export type Build = {
  Error: {} | string | null
  StartTime: string
  Log: string
  FinishTime: string
  Edits: Array<string> | null
  IsCrashRebuild: boolean
  Warnings: Array<string> | null
}

export type TiltBuild = {
  Version: string
  Date: string
  Dev: boolean
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
  Name: string
  CombinedLog: string
  BuildHistory: Array<any>
  CrashLog: string
  CurrentBuild: any
  DirectoriesWatched: Array<any>
  Endpoints: Array<string>
  PodID: string
  IsTiltfile: boolean
  LastDeployTime: string
  PathsWatched: Array<string>
  PendingBuildEdits: Array<string>
  PendingBuildReason: number
  PendingBuildSince: string
  K8sResourceInfo?: K8sResourceInfo
  DCResourceInfo?: DCResourceInfo
  RuntimeStatus: string
  TriggerMode: TriggerMode
  HasPendingChanges: boolean
  Alerts: Array<Alert>
}
export type K8sResourceInfo = {
  PodName: string
  PodCreationTime: string
  PodUpdateStartTime: string
  PodStatus: string
  PodStatusMessage: string
  PodRestarts: number
  PodLog: string
  Endpoints: Array<string>
}
export type DCResourceInfo = {
  ConfigPaths: Array<string>
  ContainerStatus: string
  ContainerID: string
  Log: string
  StartTime: string
}

export type Snapshot = {
  // input of snapshot_storage
  Message: string
  View: {
    Resources: Array<Resource>
    Log: string
    LogTimestamps: boolean
    NeedsAnalyticsNudge: boolean
    RunningTiltBuild: TiltBuild
    LatestTiltBuild: TiltBuild
    FeatureFlags: { [featureFlag: string]: boolean }
  } | null
  IsSidebarClosed: boolean
  SnapshotLink: string
  showSnapshotModal: boolean
  path?: string
  snapshotHighlight?: SnapshotHighlight | null
}

export type SnapshotHighlight = {
  beginningLogID: string
  endingLogID: string
}

export enum ShowFatalErrorModal {
  Default,
  Show,
  Hide,
}
