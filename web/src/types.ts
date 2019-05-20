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
  IsCrashOnly: boolean
}

export type TiltBuild = {
  Version: string
  Date: string
  Dev: boolean
}
