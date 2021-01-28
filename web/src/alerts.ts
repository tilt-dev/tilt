import { podStatusIsCrash, podStatusIsError } from "./constants"
import { FilterLevel, FilterSource } from "./logfilters"
import { logLinesToString } from "./logs"
import LogStore from "./LogStore"

type Resource = Proto.webviewResource
type K8sResourceInfo = Proto.webviewK8sResourceInfo

export type Alert = {
  // TODO(nick): alertType is largely obsolete now that we have
  // source and level
  alertType: string

  source: FilterSource
  level: FilterLevel

  header: string
  msg: string
  timestamp: string
  resourceName: string
  dismissHandler?: () => void
}

export const PodRestartErrorType = "PodRestartError"
export const PodStatusErrorType = "PodStatusError"
export const CrashRebuildErrorType = "ResourceCrashRebuild"
export const BuildFailedErrorType = "BuildError"
export const WarningErrorType = "Warning"

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

// Errors for both DC and K8s Resources
function buildFailed(resource: Resource) {
  let history = resource.buildHistory ?? []
  return history.length > 0 && history[0].error
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

  if (buildFailed(r)) {
    result.push(buildFailedAlert(r, logStore))
  }

  let bwa = buildWarningsAlerts(r)
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
  let msg = ""
  if (podStatusIsCrash(podStatus)) {
    msg = resource.crashLog ?? ""
  }
  msg = msg || podStatusMessage || `Pod has status ${podStatus}`

  return {
    alertType: PodStatusErrorType,
    header: "",
    msg: msg,
    timestamp: rInfo.podCreationTime ?? "",
    resourceName: resource.name ?? "",
    level: FilterLevel.error,
    source: FilterSource.runtime,
  }
}
function podRestartAlert(r: Resource): Alert {
  let rInfo = r.k8sResourceInfo as K8sResourceInfo
  let msg = r.crashLog || ""
  let header = `Restarts: ${Number(rInfo.podRestarts).toString()}`

  let dismissHandler = () => {
    let url = "/api/action"
    let payload = {
      type: "PodResetRestarts",
      manifest_name: r.name,
      visible_restarts: Number(rInfo.podRestarts),
      pod_id: rInfo.podName,
    }
    fetch(url, {
      method: "POST",
      body: JSON.stringify(payload),
      headers: {
        "Content-Type": "application/json",
      },
    }).then((response) => {
      if (!response.ok) {
        console.error(response)
      }
    })
  }

  return {
    alertType: PodRestartErrorType,
    header: header,
    msg: msg,
    timestamp: rInfo.podCreationTime ?? "",
    resourceName: r.name ?? "",
    dismissHandler: dismissHandler,
    level: FilterLevel.warn,
    source: FilterSource.runtime,
  }
}
function crashRebuildAlert(r: Resource): Alert {
  let rInfo = r.k8sResourceInfo as K8sResourceInfo
  let msg = r.crashLog || ""
  return {
    alertType: CrashRebuildErrorType,
    header: "Pod crashed",
    msg: msg,
    timestamp: rInfo.podCreationTime ?? "",
    resourceName: r.name ?? "",
    level: FilterLevel.error,
    source: FilterSource.runtime,
  }
}
function buildFailedAlert(
  resource: Resource,
  logStore: LogStore | null
): Alert {
  // both: DCResource and K8s Resource
  let history = resource.buildHistory ?? []
  let spanId = history[0].spanId || ""
  let msg = "[missing error message]"
  if (spanId && logStore) {
    msg = logLinesToString(logStore.spanLog([spanId]), false)
  }
  return {
    alertType: BuildFailedErrorType,
    header: "Build error",
    msg: msg,
    timestamp: history[0].finishTime ?? "",
    resourceName: resource.name ?? "",
    level: FilterLevel.error,
    source: FilterSource.build,
  }
}

function buildWarningsAlerts(resource: Resource): Alert[] {
  let warnings: string[] = []
  let alertArray: Alert[] = []
  let history = resource.buildHistory ?? []

  if (history.length) {
    warnings = history[0].warnings ?? []
  }
  if (warnings.length > 0) {
    warnings.forEach((w) => {
      alertArray.push({
        alertType: WarningErrorType,
        header: resource.name ?? "",
        msg: w,
        timestamp: history[0].finishTime ?? "",
        resourceName: resource.name ?? "",
        level: FilterLevel.warn,
        source: FilterSource.build,
      })
    })
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
