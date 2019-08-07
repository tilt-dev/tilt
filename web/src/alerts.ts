import { Resource } from "./types"
import { podStatusIsError, podStatusIsCrash } from "./constants"

export type Alert = {
  alertType: string
  header: string
  msg: string
  timestamp: string
  resourceName: string
}

export const PodRestartErrorType = "PodRestartError"
export const PodStatusErrorType = "PodStatusError"
export const CrashRebuildErrorType = "ResourceCrashRebuild"
export const BuildFailedErrorType = "BuildError"
export const WarningErrorType = "Warning"

function hasAlert(resource: Resource) {
  return numberOfAlerts(resource) > 0
}

//These functions determine what kind of error has occurred based on information about
//the resource - return booleans
function crashRebuild(resource: Resource): boolean {
  return (
    resource.BuildHistory.length > 0 && resource.BuildHistory[0].IsCrashRebuild
  )
}

function podStatusHasError(resource: Resource) {
  let podStatus = resource.ResourceInfo.PodStatus
  let podStatusMessage = resource.ResourceInfo.PodStatusMessage
  if (podStatus == null) {
    return false
  }
  return podStatusIsError(podStatus) || podStatusMessage
}

function podRestarted(resource: Resource) {
  return resource.ResourceInfo.PodRestarts > 0
}

function buildFailed(resource: Resource) {
  return (
    resource.BuildHistory.length > 0 && resource.BuildHistory[0].Error !== null
  )
}

function numberOfAlerts(resource: Resource): number {
  return getResourceAlerts(resource).length
}

//This function determines what kind of alert should be made based on the functions defined
//above
function getResourceAlerts(r: Resource): Array<Alert> {
  let result: Array<Alert> = []

  if (podStatusHasError(r)) {
    result.push(podStatusIsErrAlert(r))
  } else if (podRestarted(r)) {
    result.push(podRestartAlert(r))
  } else if (crashRebuild(r)) {
    result.push(crashRebuildAlert(r))
  }
  if (buildFailed(r)) {
    result.push(buildFailedAlert(r))
  }
  if (warningsAlerts(r).length > 0) {
    result = result.concat(warningsAlerts(r))
  }
  return result
}

//The following functions create the alerts based on their types, since
// they use different information from the resource to contruct their messages
function podStatusIsErrAlert(resource: Resource): Alert {
  let podStatus = resource.ResourceInfo.PodStatus
  let podStatusMessage = resource.ResourceInfo.PodStatusMessage
  let msg = ""
  if (podStatusIsCrash(podStatus)) {
    msg = resource.CrashLog
  }
  msg = msg || podStatusMessage || `Pod has status ${podStatus}`

  return {
    alertType: PodStatusErrorType,
    header: "",
    msg: msg,
    timestamp: resource.ResourceInfo.PodCreationTime,
    resourceName: resource.Name,
  }
}

function podRestartAlert(resource: Resource): Alert {
  let msg = resource.CrashLog || ""
  let header = "Restarts: "
  header = header.concat(resource.ResourceInfo.PodRestarts.toString())

  return {
    alertType: PodRestartErrorType,
    header: header,
    msg: msg,
    timestamp: resource.ResourceInfo.PodCreationTime,
    resourceName: resource.Name,
  }
}

function crashRebuildAlert(resource: Resource): Alert {
  let msg = resource.CrashLog || ""
  return {
    alertType: CrashRebuildErrorType,
    header: "Pod crashed",
    msg: msg,
    timestamp: resource.ResourceInfo.PodCreationTime,
    resourceName: resource.Name,
  }
}

function buildFailedAlert(resource: Resource): Alert {
  let msg = resource.BuildHistory[0].Log || ""
  return {
    alertType: BuildFailedErrorType,
    header: "Build error",
    msg: msg,
    timestamp: resource.BuildHistory[0].FinishTime,
    resourceName: resource.Name,
  }
}

function warningsAlerts(resource: Resource): Array<Alert> {
  let warnings: Array<string> = []
  let alertArray: Array<Alert> = []

  if (resource.BuildHistory.length > 0) {
    warnings = resource.BuildHistory[0].Warnings
  }
  if ((warnings || []).length > 0) {
    warnings.forEach(w => {
      alertArray.push({
        alertType: WarningErrorType,
        header: resource.Name,
        msg: w,
        timestamp: resource.BuildHistory[0].FinishTime,
        resourceName: resource.Name,
      })
    })
  }
  return alertArray
}

function alertKey(alert: Alert): string {
  return alert.alertType + alert.timestamp
}
export {
  getResourceAlerts,
  numberOfAlerts,
  podStatusIsErrAlert,
  warningsAlerts,
  buildFailedAlert,
  crashRebuildAlert,
  podRestartAlert,
  hasAlert,
  alertKey,
}
