import {AlertResource} from "./AlertPane";
import {Resource} from "./types";
import {podStatusIsError,podStatusIsCrash} from "./constants";

export type Alert = {
    team: string;
    alertType: string
    msg: string;
    timestamp: string;
}

const PodRestartErrorType = "PodRestartError"
const PodStatusErrorType = "PodStatusError"
const ResourceCrashRebuildErrorType = "ResourceCrashRebuild"
const BuildFailedErrorType = "BuildError"
const WarningErrorType = "Warning"

//function to assign alert type
function assignAlertType(resource: AlertResource): string{
    if (resource.podStatusIsError()){
        return PodRestartErrorType
    } else if (resource.podRestarted()){
        return PodRestartErrorType
    } else if (resource.crashRebuild()){
        return ResourceCrashRebuildErrorType
    } else if (resource.buildFailed()){
        return BuildFailedErrorType
    } else if (resource.warnings()){
        return WarningErrorType
    } else{
        return ""
    }
}

function hasAlert() {
    return alertElements([this]).length > 0
}

function crashRebuild(resource: Resource): boolean {
    return resource.BuildHistory.length > 0 && resource.BuildHistory[0].IsCrashRebuild
}

function podStatusIsError(resource: Resource) {
    let podStatus = resource.ResourceInfo.PodStatus
    let podStatusMessage = resource.ResourceInfo.PodStatusMessage
    return podStatusIsError(podStatus) || podStatusMessage
}

function podRestarted() {
    return this.resourceInfo.podRestarts > 0
}

function buildFailed() {
    return this.buildHistory.length > 0 && this.buildHistory[0].Error !== null
}

function numberOfAlerts(): number {
    return alertElements([this]).length
}

function warnings(): Array<string> {
    if (this.buildHistory.length > 0) {
    return this.buildHistory[0].Warnings || []
}

    return []
}
// //function to assign alert message based on alert type
// function assignAlertMessage(resource: <AlertResource>): string{
//
// }
//
// function assignTeam

//function to assign team

//
// Alert alert = {
//     alertType: assignAlertType(resource)
//     msg: assignAlertMessage(resource)
// }

function getAlertsResource(r: Resource): Array<Alert> {
    return []
}

export {getAlertsResource}