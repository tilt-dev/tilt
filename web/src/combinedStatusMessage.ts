import { isZeroTime } from "./time"
import { StatusItem } from "./Statusbar"
import { podStatusIsCrash, podStatusIsError } from "./constants"

const combinedStatusMessage = (resources: StatusItem[]): string => {
  let buildingResources = resources.filter(
    r => !isZeroTime(r.currentBuild.startTime)
  )

  if (buildingResources.length > 0) {
    return `Updating ${buildingResources[0].name}…`
  }

  let containerCrashedResources = resources.filter(r =>
    podStatusIsCrash(r.podStatus)
  )
  if (containerCrashedResources.length > 0) {
    return "Container crashed: " + containerCrashedResources[0].name
  }

  let resourcesWithBuildErrors = resources.filter(r => {
    return r.lastBuild && r.lastBuild.error
  })

  if (resourcesWithBuildErrors.length > 0) {
    return "Build failed: " + resourcesWithBuildErrors[0].name
  }

  let resourcesWithInterestingPodStatuses = resources.filter(
    r => podStatusIsError(r.podStatus) || r.podStatusMessage
  )
  if (resourcesWithInterestingPodStatuses.length > 0) {
    let r = resourcesWithInterestingPodStatuses[0]
    return `${r.name} has pod with status ${r.podStatus}`
  }

  return ""
}

export { combinedStatusMessage }
