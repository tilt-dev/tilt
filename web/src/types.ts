export enum ResourceView {
  Log,
  Preview,
  Alerts,
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
