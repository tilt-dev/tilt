import { isZeroTime } from "./time"
import { ResourceStatus, RuntimeStatus, TriggerMode } from "./types"

type Resource = Proto.webviewResource

function buildStatus(res: Resource): ResourceStatus {
  let currentBuild = res.currentBuild
  let hasCurrentBuild = Boolean(
    currentBuild && !isZeroTime(currentBuild.startTime)
  )
  let hasPendingBuild =
    !isZeroTime(res.pendingBuildSince) &&
    res.triggerMode === TriggerMode.TriggerModeAuto
  let buildHistory = res.buildHistory || []
  let lastBuild = buildHistory[0]
  let lastBuildError = lastBuild ? lastBuild.error : ""
  let hasWarnings = buildWarnings(res).length > 0

  if (hasCurrentBuild) {
    return ResourceStatus.Building
  } else if (hasPendingBuild) {
    return ResourceStatus.Pending
  } else if (lastBuildError) {
    return ResourceStatus.Unhealthy
  } else if (hasWarnings) {
    return ResourceStatus.Warning
  } else if (!lastBuild) {
    return ResourceStatus.None
  }
  return ResourceStatus.Healthy
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

export {
  buildStatus,
  runtimeStatus,
  combinedStatus,
  warnings,
  buildWarnings,
  runtimeWarnings,
}
