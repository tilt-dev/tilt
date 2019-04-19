import { isZeroTime } from "./time"
import { StatusItem } from "./Statusbar"

const combinedStatusMessage = (resources: Array<StatusItem>): string => {
  let buildingResources = resources.filter(
    r => !isZeroTime(r.currentBuild.StartTime)
  )

  if (buildingResources.length > 0) {
    return "Building: " + buildingResources[0].name
  }

  let containerCrashedResources = resources.filter(
    r => r.podStatus === "CrashLoopBackOff"
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

  return ""
}

export { combinedStatusMessage }
