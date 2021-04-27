import { Duration } from "moment"
import { timeDiff } from "./time"
import { ResourceStatus, RuntimeStatus, UpdateStatus } from "./types"

type Resource = Proto.webviewResource

function buildStatus(res: Resource): ResourceStatus {
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
  } else if (buildWarnings(res).length > 0) {
    return ResourceStatus.Warning
  } else if (res.updateStatus == UpdateStatus.Ok) {
    return ResourceStatus.Healthy
  }
  return ResourceStatus.None
}

function runtimeStatus(res: Resource): ResourceStatus {
  let hasWarnings = runtimeWarnings(res).length > 0
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
function combinedStatus(res: Resource): ResourceStatus {
  let bs = buildStatus(res)
  if (bs !== ResourceStatus.Healthy && bs !== ResourceStatus.None) {
    return bs
  }
  let rs = runtimeStatus(res)
  if (rs === ResourceStatus.None) {
    return bs
  }
  return rs
}

function buildWarnings(res: any): string[] {
  let buildHistory = res.buildHistory || []
  let lastBuild = buildHistory[0]
  return Array.from((lastBuild && lastBuild.warnings) || [])
}

function runtimeWarnings(res: any): string[] {
  let warnings = []
  if (res.k8sResourceInfo && res.k8sResourceInfo.podRestarts > 0) {
    warnings.push("Container restarted")
  }
  return warnings
}

function warnings(res: any): string[] {
  let warnings = buildWarnings(res)
  warnings.push(...runtimeWarnings(res))
  return warnings
}

function lastBuildDuration(res: Resource): Duration | null {
  const buildHistory = res.buildHistory || []
  const lastBuild = buildHistory.length > 0 ? buildHistory[0] : null
  if (lastBuild && lastBuild.startTime && lastBuild.finishTime) {
    return timeDiff(lastBuild.startTime, lastBuild.finishTime)
  } else {
    return null
  }
}

export {
  buildStatus,
  runtimeStatus,
  combinedStatus,
  warnings,
  buildWarnings,
  runtimeWarnings,
  lastBuildDuration,
}
