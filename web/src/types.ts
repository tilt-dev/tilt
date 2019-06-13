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

export enum ResourceStatus {
  Ok = "ok",
  Pending = "pending",
  Error = "error",
}
