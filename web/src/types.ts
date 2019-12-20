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
  TriggerModeManualAfterInitial,
  TriggerModeManualIncludingInitial,
}

// what is the status of the resource in the cluster
export enum RuntimeStatus {
  Ok = "ok",
  Pending = "pending",
  Error = "error",
  NotApplicable = "not_applicable",
}

// What is the status of the resource with respect to Tilt
export enum ResourceStatus {
  Building, // Tilt is actively doing something (e.g., docker build or kubectl apply)
  Pending, // not building, healthy, or unhealthy, but presumably on its way to one of those (e.g., queued to build, or ContainerCreating)
  Healthy, // e.g., build succeeded and pod is running and healthy
  Unhealthy, // e.g., last build failed, or CrashLoopBackOff
  Warning, // e.g., an undismissed restart
  None, // e.g., a manual build that has never executed
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

export type Snapshot = {
  view: Proto.webviewView
  isSidebarClosed: boolean
  path?: string
  snapshotHighlight?: SnapshotHighlight | null
}

// A plaintext representation of a line of the log,
// with metadata to render it in isolation.
//
// The metadata should be stored as primitive fields
// so that React's default caching behavior will kick in.
export type LogLine = {
  // We assume that 'text' does not contain a newline
  text: string
  manifestName: string
  level: string
}
