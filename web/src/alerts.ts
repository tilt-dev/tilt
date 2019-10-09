import { K8sResourceInfo, Resource } from "./types"
import { podStatusIsError, podStatusIsCrash } from "./constants"

export type Alert = {
  alertType: string
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
  let rInfo = r.K8sResourceInfo as K8sResourceInfo
  let podStatus = rInfo.PodStatus
  let podStatusMessage = rInfo.PodStatusMessage
  if (podStatus == null) {
    return false
  }
  return podStatusIsError(podStatus) || podStatusMessage
}

function podRestarted(r: Resource) {
  let rInfo = r.K8sResourceInfo as K8sResourceInfo
  return rInfo.PodRestarts > 0
}

// Errors for both DC and K8s Resources
function buildFailed(resource: Resource) {
  return (
    resource.BuildHistory.length > 0 && resource.BuildHistory[0].Error !== null
  )
}

//This function determines what kind of alert should be made based on the functions defined
//above
function getResourceAlerts(r: Resource): Array<Alert> {
  let result: Array<Alert> = []

  if (r.K8sResourceInfo) {
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
  let rInfo = resource.K8sResourceInfo as K8sResourceInfo
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
  let rInfo = r.K8sResourceInfo as K8sResourceInfo
  let msg = r.CrashLog || ""
  let header = `Restarts: ${rInfo.PodRestarts.toString()}`

  let dismissHandler = () => {
    let url = "/api/action"
    let payload = {
      type: "PodResetRestarts",
      manifest_name: r.Name,
      visible_restarts: Number(rInfo.PodRestarts),
      pod_id: rInfo.PodName,
    }
    fetch(url, {
      method: "POST",
      body: JSON.stringify(payload),
      headers: {
        "Content-Type": "application/json",
      },
    }).then(response => {
      if (!response.ok) {
        console.error(response)
      }
    })
  }

  return {
    alertType: PodRestartErrorType,
    header: header,
    msg: msg,
    timestamp: rInfo.PodCreationTime,
    resourceName: r.Name,
    dismissHandler: dismissHandler,
  }
}
function crashRebuildAlert(r: Resource): Alert {
  let rInfo = r.K8sResourceInfo as K8sResourceInfo
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
}
