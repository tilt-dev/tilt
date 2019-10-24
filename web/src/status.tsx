import { isZeroTime } from "./time"
import { RuntimeStatus, ResourceStatus, Resource } from "./types"

// A combination of runtime status and build status over a resource view.
// 1) If there's a current or pending build, this is "pending".
// 2) Otherwise, if there's a build error or runtime error, this is "error".
// 3) Otherwise, we fallback to runtime status.
function combinedStatus(res: Resource): RuntimeStatus {
  let status = res.runtimeStatus
  let currentBuild = res.currentBuild
  let hasCurrentBuild = Boolean(
    currentBuild && !isZeroTime(currentBuild.startTime)
  )
  let hasPendingBuild = !isZeroTime(res.pendingBuildSince)
  let buildHistory = res.buildHistory || []
  let lastBuild = buildHistory[0]
  let lastBuildError = lastBuild ? lastBuild.error : ""

  if (hasCurrentBuild || hasPendingBuild) {
    return RuntimeStatus.Pending
  }
  if (lastBuildError) {
    return RuntimeStatus.Error
  }

  switch (status) {
    case RuntimeStatus.Error:
      return RuntimeStatus.Error
    case RuntimeStatus.Pending:
      return RuntimeStatus.Pending
    case RuntimeStatus.Ok:
      return RuntimeStatus.Ok
    default:
      return RuntimeStatus.Unknown
  }
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

function tiltStatus(res: Resource): ResourceStatus {
  return ResourceStatus.Building
}

export { combinedStatus, warnings, tiltStatus }
