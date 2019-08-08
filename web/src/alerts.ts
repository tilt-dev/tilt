import { DCResourceInfo, K8sResourceInfo, Resource } from "./types"
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
function crashRebuild(r: Resource): boolean {
  if (r.ResourceInfo.type !== "DCResource") {
    return r.BuildHistory.length > 0 && r.BuildHistory[0].IsCrashRebuild
  } else {
    return false
  }
}

function podStatusHasError(r: Resource) {
  if (r.ResourceInfo.type !== "DCResource") {
    let podStatus = r.ResourceInfo.PodStatus
    let podStatusMessage = r.ResourceInfo.PodStatusMessage
    if (podStatus == null) {
      return false
    }
    return podStatusIsError(podStatus) || podStatusMessage
  } else {
    return false
  }
}

function podRestarted(r: Resource) {
  return r.ResourceInfo.type !== "DCResource"
    ? r.ResourceInfo.PodRestarts > 0
    : false
}

function buildFailed(resource: Resource) {
  return (
    resource.BuildHistory.length > 0 && resource.BuildHistory[0].Error !== null
  )
}

//This function determines what kind of alert should be made based on the functions defined
//above
function getResourceAlerts(r: Resource): Array<Alert> {
  let result: Array<Alert> = []
  if (podStatusHasError(r)) {
    result.push(podStatusIsErrAlert(r))
  } else if (podRestarted(r)) {
    result.push(podRestartAlert(r))
  }
  if (crashRebuild(r)) {
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

function numberOfAlerts(resource: Resource): number {
  return getResourceAlerts(resource).length
}

//The following functions create the alerts based on their types, since
// they use different information from the resource to contruct their messages
function podStatusIsErrAlert(resource: Resource): Alert {
  // K8s resource
  if (resource.ResourceInfo.type !== "DCResource") {
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
  } else {
    return {
      // returns this is DCResource - never gets here because podStatusIsError is false for DCResource
      alertType: "",
      header: "",
      msg: "",
      timestamp: "",
      resourceName: "",
    }
  }
}
function podRestartAlert(resource: Resource): Alert {
  // K8s resource
  if (resource.ResourceInfo.type !== "DCResource") {
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
  } else {
    return emptyAlert()
  }
}
function crashRebuildAlert(resource: Resource): Alert {
  // K8s resource
  if (resource.ResourceInfo.type !== "DCResource") {
    let msg = resource.CrashLog || ""
    return {
      alertType: CrashRebuildErrorType,
      header: "Pod crashed",
      msg: msg,
      timestamp: resource.BuildHistory[0].FinishTime,
      resourceName: resource.Name,
    }
  } else {
    return emptyAlert()
  }
}
function buildFailedAlert(resource: Resource): Alert {
  // both: DCResource and K8s Resource
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
  // both: DCResource and K8sResource
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

function emptyAlert(): Alert {
  return {
    alertType: "",
    header: "",
    msg: "",
    timestamp: "",
    resourceName: "",
  }
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
