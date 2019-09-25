import { isZeroTime } from "./time"
import { RuntimeStatus, ResourceStatus, Resource } from "./types"

// A combination of runtime status and build status over a resource view.
// 1) If there's a current or pending build, this is "pending".
// 2) Otherwise, if there's a build error or runtime error, this is "error".
// 3) Otherwise, we fallback to runtime status.
function combinedStatus(res: Resource): RuntimeStatus {
  let status = res.RuntimeStatus
  let currentBuild = res.CurrentBuild
  let hasCurrentBuild = Boolean(
    currentBuild && !isZeroTime(currentBuild.StartTime)
  )
  let hasPendingBuild = !isZeroTime(res.PendingBuildSince)
  let buildHistory = res.BuildHistory || []
  let lastBuild = buildHistory[0]
  let lastBuildError = lastBuild ? lastBuild.Error : ""

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
  let buildHistory = res.BuildHistory || []
  let lastBuild = buildHistory[0]
  let warnings = (lastBuild && lastBuild.Warnings) || []
  warnings = Array.from(warnings)

  if (res.K8sResourceInfo && res.K8sResourceInfo.PodRestarts > 0) {
    warnings.push("Container restarted")
  }

  return warnings
}

function tiltStatus(res: Resource): ResourceStatus {
  return ResourceStatus.Building
}

export { combinedStatus, warnings, tiltStatus }
