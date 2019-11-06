import { isZeroTime } from "./time"
import { Resource, ResourceStatus, RuntimeStatus, TriggerMode } from "./types"

// A combination of runtime status and build status over a resource view.
// 1) If there's a current or pending build, this is "pending".
// 2) Otherwise, if there's a build error or runtime error, this is "error".
// 3) Otherwise, we fallback to runtime status.
function combinedStatus(res: Resource): ResourceStatus {
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

  if (hasCurrentBuild) {
    return ResourceStatus.Building
  }
  if (hasPendingBuild) {
    return ResourceStatus.Pending
  }
  if (lastBuildError) {
    return ResourceStatus.Unhealthy
  }

  switch (res.runtimeStatus) {
    case RuntimeStatus.Error:
      return ResourceStatus.Unhealthy
    case RuntimeStatus.Pending:
      return ResourceStatus.Pending
    case RuntimeStatus.Ok:
      return ResourceStatus.Healthy
    case RuntimeStatus.NotApplicable:
      if (res.buildHistory.length > 0) {
        return ResourceStatus.Healthy
      } else {
        return ResourceStatus.None
      }
  }
  return ResourceStatus.None
}

function warnings(res: any): string[] {
  let buildHistory = res.buildHistory || []
  let lastBuild = buildHistory[0]
  let warnings = (lastBuild && lastBuild.warnings) || []
  warnings = Array.from(warnings)

  if (res.k8sResourceInfo && res.k8sResourceInfo.podRestarts > 0) {
    warnings.push("Container restarted")
  }

  return warnings
}

export { combinedStatus, warnings }
