import { Resource } from "./types"
import { podStatusIsError, podStatusIsCrash } from "./constants"
import React from "react"

export type Alert = {
  alertType: string
  msg: string
  timestamp: string
  titleMsg: string
}

export const PodRestartErrorType = "PodRestartError"
export const PodStatusErrorType = "PodStatusError"
export const ResourceCrashRebuildErrorType = "ResourceCrashRebuild"
export const BuildFailedErrorType = "BuildError"
export const WarningErrorType = "Warning"

//these functions can be moved to where the resources type is defined
function hasAlert(resource: Resource) {
  return numberOfAlerts(resource) > 0
}

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
function getResourceAlerts(r: Resource): Array<Alert> {
  let result: Array<Alert> = []

  if (podStatusHasError(r)) {
    result.push(podStatusErrAlert(r))
  } else if (podRestarted(r)) {
    result.push(podRestartErrAlert(r))
  } else if (crashRebuild(r)) {
    result.push(crashRebuildErrAlert(r))
  }
  if (buildFailed(r)) {
    result.push(buildFailedErrAlert(r))
  }
  if (warningsErrAlerts(r).length > 0) {
    result = result.concat(warningsErrAlerts(r))
  }
  return result
}

// The following functions are based on the current type of errors that we showed in "AlertPane.tsx"
// that classifies the different errors, the following functions create the alerts based on their types, since
// they displayed different messages
function podStatusErrAlert(resource: Resource): Alert {
  let podStatus = resource.ResourceInfo.PodStatus
  let podStatusMessage = resource.ResourceInfo.PodStatusMessage
  let msg = ""
  if (podStatusIsCrash(podStatus)) {
    msg = resource.CrashLog
  }
  msg = msg || podStatusMessage || `Pod has status ${podStatus}`

  return {
    alertType: PodStatusErrorType,
    titleMsg: "",
    msg: msg,
    timestamp: resource.ResourceInfo.PodCreationTime,
  }
}

function podRestartErrAlert(resource: Resource): Alert {
  let msg = resource.CrashLog || ""
  let titleMsg = "Restarts: "
  titleMsg = titleMsg.concat(resource.ResourceInfo.PodRestarts.toString())

  return {
    alertType: PodRestartErrorType,
    titleMsg: titleMsg,
    msg: msg,
    timestamp: resource.ResourceInfo.PodCreationTime,
  }
}

function crashRebuildErrAlert(resource: Resource): Alert {
  let msg = resource.CrashLog || ""
  return {
    alertType: ResourceCrashRebuildErrorType,
    titleMsg: "Pod crashed",
    msg: msg,
    timestamp: resource.ResourceInfo.PodCreationTime,
  }
}

function buildFailedErrAlert(resource: Resource): Alert {
  let msg = resource.BuildHistory[0].Log || ""
  return {
    alertType: BuildFailedErrorType,
    titleMsg: "Build error",
    msg: msg,
    timestamp: resource.ResourceInfo.PodCreationTime,
  }
}
function warningsErrAlerts(resource: Resource): Array<Alert> {
  let warnings: Array<string> = []
  let alertArray: Array<Alert> = []

  if (resource.BuildHistory.length > 0) {
    warnings = resource.BuildHistory[0].Warnings
  }
  if ((warnings || []).length > 0) {
    warnings.forEach(w => {
      alertArray.push({
        alertType: WarningErrorType,
        titleMsg: resource.Name,
        msg: w,
        timestamp: resource.BuildHistory[0].FinishTime,
      })
    })
  }
  return alertArray
}
export {
  getResourceAlerts,
  numberOfAlerts,
  podStatusErrAlert,
  warningsErrAlerts,
  buildFailedErrAlert,
  crashRebuildErrAlert,
  podRestartErrAlert,
}
