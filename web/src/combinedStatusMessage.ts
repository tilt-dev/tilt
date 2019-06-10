import { isZeroTime } from "./time"
import { StatusItem } from "./Statusbar"
import {
  podStatusCrashLoopBackOff,
  podStatusError,
  podStatusImagePullBackOff,
  podStatusErrImgPull,
} from "./constants"

const combinedStatusMessage = (resources: Array<StatusItem>): string => {
  let buildingResources = resources.filter(
    r => !isZeroTime(r.currentBuild.StartTime)
  )

  if (buildingResources.length > 0) {
    return `Updating ${buildingResources[0].name}…`
  }

  let containerCrashedResources = resources.filter(
    r =>
      r.podStatus === podStatusCrashLoopBackOff ||
      r.podStatus === podStatusError
  )
  if (containerCrashedResources.length > 0) {
    return "Container crashed: " + containerCrashedResources[0].name
  }

  let resourcesWithBuildErrors = resources.filter(r => {
    return r.lastBuild && r.lastBuild.Error
  })

  if (resourcesWithBuildErrors.length > 0) {
    return "Build failed: " + resourcesWithBuildErrors[0].name
  }

  let resourcesWithInterestingPodStatuses = resources.filter(
    r =>
      r.podStatus === podStatusImagePullBackOff ||
      r.podStatus === podStatusErrImgPull
  )
  if (resourcesWithInterestingPodStatuses.length > 0) {
    let r = resourcesWithInterestingPodStatuses[0]
    return `${r.name} has pod with status ${r.podStatus}`
  }

  return ""
}

export { combinedStatusMessage }
