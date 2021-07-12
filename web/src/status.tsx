import { ResourceStatus, RuntimeStatus, UpdateStatus } from "./types"

type UIResource = Proto.v1alpha1UIResource
type UIResourceStatus = Proto.v1alpha1UIResourceStatus

function buildStatus(r: UIResource): ResourceStatus {
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
  } else if (buildWarnings(r).length > 0) {
    return ResourceStatus.Warning
  } else if (res.updateStatus == UpdateStatus.Ok) {
    return ResourceStatus.Healthy
  }
  return ResourceStatus.None
}

function runtimeStatus(r: UIResource): ResourceStatus {
  let res = r.status || {}
  let hasWarnings = runtimeWarnings(r).length > 0
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
function summaryStatus(res: UIResource): ResourceStatus {
  return summaryStatusFromAllStatuses({
    buildStatus: buildStatus(res),
    runtimeStatus: runtimeStatus(res),
  })
}

function summaryStatusFromAllStatuses({
  buildStatus,
  runtimeStatus,
}: {
  buildStatus: ResourceStatus
  runtimeStatus: ResourceStatus
}) {
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

function buildWarnings(res: UIResource): string[] {
  let buildHistory = res.status?.buildHistory || []
  let lastBuild = buildHistory[0]
  return Array.from((lastBuild && lastBuild.warnings) || [])
}

function runtimeWarnings(res: UIResource): string[] {
  let warnings = []
  let podRestarts = res.status?.k8sResourceInfo?.podRestarts
  if (podRestarts && podRestarts > 0) {
    warnings.push("Container restarted")
  }
  return warnings
}

function warnings(res: UIResource): string[] {
  let warnings = buildWarnings(res)
  warnings.push(...runtimeWarnings(res))
  return warnings
}

export {
  buildStatus,
  runtimeStatus,
  summaryStatus,
  summaryStatusFromAllStatuses,
  warnings,
  buildWarnings,
  runtimeWarnings,
}
