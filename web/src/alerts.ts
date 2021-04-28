import { podStatusIsError } from "./constants"
import { FilterLevel, FilterSource } from "./logfilters"
import { logLinesToString } from "./logs"
import LogStore from "./LogStore"

type Resource = Proto.webviewResource
type K8sResourceInfo = Proto.webviewK8sResourceInfo

export type Alert = {
  source: FilterSource
  level: FilterLevel

  msg: string
  resourceName: string
}

//These functions determine what kind of error has occurred based on information about
//the resource - return booleans

//Errors for K8s Resources
function crashRebuild(r: Resource): boolean {
  let history = r.buildHistory ?? []
  return history.length > 0 && !!history[0].isCrashRebuild
}

function podStatusHasError(r: Resource) {
  let rInfo = r.k8sResourceInfo as K8sResourceInfo
  let podStatus = rInfo.podStatus
  let podStatusMessage = rInfo.podStatusMessage
  if (podStatus == null) {
    return false
  }
  return podStatusIsError(podStatus) || podStatusMessage
}

function podRestarted(r: Resource) {
  let rInfo = r.k8sResourceInfo as K8sResourceInfo
  return (rInfo.podRestarts ?? 0) > 0
}

function runtimeAlerts(r: Resource, logStore: LogStore | null): Alert[] {
  let result: Alert[] = []

  if (r.k8sResourceInfo) {
    // K8s alerts
    if (podStatusHasError(r)) {
      result.push(podStatusIsErrAlert(r))
    } else if (podRestarted(r)) {
      result.push(podRestartAlert(r))
    }
    if (crashRebuild(r)) {
      result.push(crashRebuildAlert(r))
    }
  }

  return result
}

function buildAlerts(r: Resource, logStore: LogStore | null): Alert[] {
  let result: Alert[] = []

  const failAlert = buildFailedAlert(r, logStore)
  if (failAlert) {
    result.push(failAlert)
  }

  let bwa = buildWarningsAlerts(r, logStore)
  if (bwa.length > 0) {
    result.push(...bwa)
  }
  return result
}

function combinedAlerts(r: Resource, logStore: LogStore | null): Alert[] {
  let result = runtimeAlerts(r, logStore)
  result.push(...buildAlerts(r, logStore))
  return result
}

//The following functions create the alerts based on their types, since
// they use different information from the resource to contruct their messages
function podStatusIsErrAlert(resource: Resource): Alert {
  let rInfo = resource.k8sResourceInfo as K8sResourceInfo
  let podStatus = rInfo.podStatus
  let podStatusMessage = rInfo.podStatusMessage
  let msg = podStatusMessage || `Pod has status ${podStatus}`

  return {
    msg: msg,
    resourceName: resource.name ?? "",
    level: FilterLevel.error,
    source: FilterSource.runtime,
  }
}
function podRestartAlert(r: Resource): Alert {
  let rInfo = r.k8sResourceInfo as K8sResourceInfo
  let msg = `Restarts: ${Number(rInfo.podRestarts).toString()}`
  return {
    msg: msg,
    resourceName: r.name ?? "",
    level: FilterLevel.warn,
    source: FilterSource.runtime,
  }
}
function crashRebuildAlert(r: Resource): Alert {
  let rInfo = r.k8sResourceInfo as K8sResourceInfo
  let msg = "Pod crashed"
  return {
    msg: msg,
    resourceName: r.name ?? "",
    level: FilterLevel.error,
    source: FilterSource.runtime,
  }
}
function buildFailedAlert(
  resource: Resource,
  logStore: LogStore | null
): Alert | null {
  // both: DCResource and K8s Resource
  const latestBuild = (resource.buildHistory ?? [])[0]
  const spanId = latestBuild?.spanId ?? ""
  if (
    !latestBuild ||
    !latestBuild.error ||
    (logStore && !logStore.hasLinesForSpan(spanId))
  ) {
    return null
  }
  let msg = "[missing error message]"
  if (logStore) {
    msg = logLinesToString(logStore.spanLog([spanId]), false)
  }
  return {
    msg: msg,
    resourceName: resource.name ?? "",
    level: FilterLevel.error,
    source: FilterSource.build,
  }
}

function buildWarningsAlerts(
  resource: Resource,
  logStore: LogStore | null
): Alert[] {
  let alertArray: Alert[] = []
  const latestBuild = (resource.buildHistory ?? [])[0]
  if (
    latestBuild &&
    (!logStore || logStore.hasLinesForSpan(latestBuild.spanId ?? ""))
  ) {
    alertArray = (latestBuild.warnings || []).map((w) => ({
      msg: w,
      resourceName: resource.name ?? "",
      level: FilterLevel.warn,
      source: FilterSource.build,
    }))
  }
  return alertArray
}

export {
  combinedAlerts,
  buildAlerts,
  runtimeAlerts,
  podStatusIsErrAlert,
  buildWarningsAlerts,
  buildFailedAlert,
  crashRebuildAlert,
  podRestartAlert,
}
