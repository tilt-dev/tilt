import { numberOfAlerts } from "./alerts"
import { ResourceStatus, TriggerMode } from "./types"
import { combinedStatus } from "./status"

const moment = require("moment")

type Resource = Proto.webviewResource
type Build = Proto.webviewBuildRecord

function timeDiff(start: string, end: string): moment.Duration {
  let t1 = moment(start)
  let t2 = moment(end)
  return moment.duration(t2.diff(t1))
}

class SidebarItem {
  name: string
  isTiltfile: boolean
  status: ResourceStatus
  hasEndpoints: boolean
  lastBuildDur: moment.Duration | null
  lastDeployTime: string
  pendingBuildSince: string
  currentBuildStartTime: string
  alertCount: number
  triggerMode: TriggerMode
  hasPendingChanges: boolean
  queued: boolean
  lastBuild: Build | null = null

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: Resource) {
    let buildHistory = res.buildHistory || []
    let lastBuild = buildHistory.length > 0 ? buildHistory[0] : null

    this.name = res.name ?? ""
    this.isTiltfile = !!res.isTiltfile
    this.status = combinedStatus(res)
    this.hasEndpoints = (res.endpointLinks || []).length > 0
    this.lastBuildDur =
      lastBuild && lastBuild.startTime && lastBuild.finishTime
        ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
        : null
    this.lastDeployTime = res.lastDeployTime ?? ""
    this.pendingBuildSince = res.pendingBuildSince ?? ""
    this.currentBuildStartTime = res.currentBuild?.startTime ?? ""
    this.alertCount = numberOfAlerts(res)
    this.triggerMode = res.triggerMode ?? TriggerMode.TriggerModeAuto
    this.hasPendingChanges = !!res.hasPendingChanges
    this.queued = !!res.queued
    this.lastBuild = lastBuild
  }
}

export default SidebarItem
