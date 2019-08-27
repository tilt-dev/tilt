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

//Errors for K8s Resources
function crashRebuild(r: Resource): boolean {
  return r.BuildHistory.length > 0 && r.BuildHistory[0].IsCrashRebuild
}

function podStatusHasError(r: Resource) {
  let rInfo = <K8sResourceInfo>r.ResourceInfo
  let podStatus = rInfo.PodStatus
  let podStatusMessage = rInfo.PodStatusMessage
  if (podStatus == null) {
    return false
  }
  return podStatusIsError(podStatus) || podStatusMessage
}

function podRestarted(r: Resource) {
  let rInfo = <K8sResourceInfo>r.ResourceInfo
  return rInfo.PodRestarts > 0
}

// Errors for both DC and K8s Resources
function buildFailed(resource: Resource) {
  return (
    resource.BuildHistory.length > 0 && resource.BuildHistory[0].Error !== null
  )
}

function isK8sResourceInfo(
  resourceInfo: K8sResourceInfo | DCResourceInfo
): resourceInfo is K8sResourceInfo {
  return (<K8sResourceInfo>resourceInfo).PodName !== undefined
}

//This function determines what kind of alert should be made based on the functions defined
//above
function getResourceAlerts(r: Resource): Array<Alert> {
  let result: Array<Alert> = []

  if (isK8sResourceInfo(r.ResourceInfo)) {
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
  let rInfo = <K8sResourceInfo>resource.ResourceInfo
  let podStatus = rInfo.PodStatus
  let podStatusMessage = rInfo.PodStatusMessage
  let msg = ""
  if (podStatusIsCrash(podStatus)) {
    msg = resource.CrashLog
  }
  msg = msg || podStatusMessage || `Pod has status ${podStatus}`

  return {
    alertType: PodStatusErrorType,
    header: "",
    msg: msg,
    timestamp: rInfo.PodCreationTime,
    resourceName: resource.Name,
  }
}
function podRestartAlert(r: Resource): Alert {
  let rInfo = <K8sResourceInfo>r.ResourceInfo
  let msg = r.CrashLog || ""
  let header = "Restarts: "
  header = header.concat(rInfo.PodRestarts.toString())
  return {
    alertType: PodRestartErrorType,
    header: header,
    msg: msg,
    timestamp: rInfo.PodCreationTime,
    resourceName: r.Name,
  }
}
function crashRebuildAlert(r: Resource): Alert {
  let rInfo = <K8sResourceInfo>r.ResourceInfo
  let msg = r.CrashLog || ""
  return {
    alertType: CrashRebuildErrorType,
    header: "Pod crashed",
    msg: msg,
    timestamp: rInfo.PodCreationTime,
    resourceName: r.Name,
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

export {
  getResourceAlerts,
  numberOfAlerts,
  podStatusIsErrAlert,
  warningsAlerts,
  buildFailedAlert,
  crashRebuildAlert,
  podRestartAlert,
  hasAlert,
  isK8sResourceInfo,
}
