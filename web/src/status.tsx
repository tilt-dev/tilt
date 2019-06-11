import { isZeroTime } from "./time"
import { ResourceStatus } from "./types"

// A combination of runtime status and build status over a resource view.
// 1) If there's a current or pending build, this is "pending".
// 2) Otherwise, if there's a build error or runtime error, this is "error".
// 3) Otherwise, we fallback to runtime status.
function combinedStatus(res: any): ResourceStatus {
  let runtimeStatus: ResourceStatus = res.RuntimeStatus
  let currentBuild = res.CurrentBuild
  let hasCurrentBuild = Boolean(
    currentBuild && !isZeroTime(currentBuild.StartTime)
  )
  let hasPendingBuild = !isZeroTime(res.PendingBuildSince)
  let buildHistory = res.BuildHistory || []
  let lastBuild = buildHistory[0]
  let lastBuildError = lastBuild ? lastBuild.Error : ""

  if (hasCurrentBuild || hasPendingBuild) {
    return ResourceStatus.Pending
  }
  if (lastBuildError) {
    return ResourceStatus.Error
  }
  return runtimeStatus
}

function warnings(res: any): string[] {
  let buildHistory = res.BuildHistory || []
  let lastBuild = buildHistory[0]
  let warnings = (lastBuild && lastBuild.Warnings) || []
  warnings = Array.from(warnings)

  if (res.ResourceInfo && res.ResourceInfo.PodRestarts > 0) {
    warnings.push("Container restarted")
  }

  return warnings
}

export { combinedStatus, warnings }
