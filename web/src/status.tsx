import { buildWarningCount, runtimeWarningCount } from "./alerts"
import { Hold } from "./Hold"
import { LogAlertIndex } from "./LogStore"
import { ResourceStatus, RuntimeStatus, UpdateStatus } from "./types"

type UIResource = Proto.v1alpha1UIResource
type UIResourceStatus = Proto.v1alpha1UIResourceStatus

function buildStatus(r: UIResource, alertIndex: LogAlertIndex): ResourceStatus {
  let res = r.status || {}
  if (res.updateStatus == UpdateStatus.InProgress) {
    return ResourceStatus.Building
  } else if (res.updateStatus == UpdateStatus.Pending) {
    return ResourceStatus.Pending
  } else if (
    res.updateStatus == UpdateStatus.NotApplicable ||
    res.updateStatus == UpdateStatus.None
  ) {
    return ResourceStatus.None
  } else if (res.updateStatus == UpdateStatus.Error) {
    return ResourceStatus.Unhealthy
  } else if (buildWarningCount(r, alertIndex) > 0) {
    // Warnings are derived from the log store, so that clearing
    // logs clears the warning indicator.
    return ResourceStatus.Warning
  } else if (res.updateStatus == UpdateStatus.Ok) {
    return ResourceStatus.Healthy
  }
  return ResourceStatus.None
}

function runtimeStatus(
  r: UIResource,
  alertIndex: LogAlertIndex
): ResourceStatus {
  let res = r.status || {}

  // Warnings are derived from the log store, so that clearing
  // logs clears the warning indicator.
  let hasWarnings = runtimeWarningCount(r, alertIndex) > 0
  if (hasWarnings) {
    if (res.runtimeStatus === RuntimeStatus.Error) {
      return ResourceStatus.Unhealthy
    } else {
      return ResourceStatus.Warning
    }
  }

  switch (res.runtimeStatus) {
    case RuntimeStatus.Error:
      return ResourceStatus.Unhealthy
    case RuntimeStatus.Pending:
      return ResourceStatus.Pending
    case RuntimeStatus.Ok:
      return ResourceStatus.Healthy
    case RuntimeStatus.NotApplicable:
      return ResourceStatus.None
  }
  return ResourceStatus.None
}

// A combination of runtime status and build status over a resource view.
// 1) If there's a current or pending build, this is "pending".
// 2) Otherwise, if there's a build error or runtime error, this is "error".
// 3) Otherwise, we fallback to runtime status.
function combinedStatus(
  buildStatus: ResourceStatus,
  runtimeStatus: ResourceStatus
): ResourceStatus {
  if (
    buildStatus !== ResourceStatus.Healthy &&
    buildStatus !== ResourceStatus.None
  ) {
    return buildStatus
  }

  if (runtimeStatus === ResourceStatus.None) {
    return buildStatus
  }

  return runtimeStatus
}

export function PendingBuildDescription(hold?: Hold | null): string {
  let text = "Update: "
  if (!hold?.count) {
    text += "pending"
    return text
  }

  text += "waiting on "
  const maxToShow = 3
  let toShow: string[] = []
  if (hold?.images.length) {
    text += hold.images.length > 1 ? "images: " : "image: "
    toShow = hold.images
  } else if (hold?.resources.length) {
    text += hold.resources.length > 1 ? "resources: " : "resource: "
    toShow = hold.resources
  } else {
    text += `${hold.count} object${hold.count > 1 ? "s" : ""}`
    return text
  }

  text += toShow.slice(0, maxToShow).join(", ")
  if (toShow.length > maxToShow) {
    const extra = toShow.length - maxToShow
    text += `, and ${extra} more`
  }

  return text
}

export { buildStatus, runtimeStatus, combinedStatus }
