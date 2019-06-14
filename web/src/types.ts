export enum ResourceView {
  Log,
  Preview,
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
  ResourceInfo: {
    PodCreationTime: string
    PodLog: string
    PodName: string
    PodRestarts: number
    PodUpdateStartTime: string
    YAML: string
    PodStatus: string
    Endpoints: Array<string>
  }
  RuntimeStatus: string
  TriggerMode: TriggerMode
  HasPendingChanges: boolean
}
