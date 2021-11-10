import moment from "moment"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { Hold } from "./Hold"
import { getResourceLabels } from "./labels"
import { LogAlertIndex } from "./LogStore"
import { resourceIsDisabled } from "./ResourceStatus"
import { buildStatus, runtimeStatus } from "./status"
import { timeDiff } from "./time"
import {
  Build,
  ResourceName,
  ResourceStatus,
  TriggerMode,
  UIResource,
  UIResourceStatus,
} from "./types"

class SidebarItem {
  name: string
  isTiltfile: boolean
  isTest: boolean
  buildStatus: ResourceStatus
  buildAlertCount: number
  runtimeStatus: ResourceStatus
  runtimeAlertCount: number
  hasEndpoints: boolean
  labels: string[]
  lastBuildDur: moment.Duration | null
  lastDeployTime: string
  pendingBuildSince: string
  currentBuildStartTime: string
  triggerMode: TriggerMode
  hasPendingChanges: boolean
  queued: boolean
  lastBuild: Build | null = null
  hold: Hold | null = null
  disabled: boolean

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: UIResource, logAlertIndex: LogAlertIndex) {
    let status = (res.status || {}) as UIResourceStatus
    let buildHistory = status.buildHistory || []
    let lastBuild = buildHistory.length > 0 ? buildHistory[0] : null
    this.name = res.metadata?.name ?? ""
    this.isTiltfile = this.name === ResourceName.tiltfile
    this.isTest = !!status.localResourceInfo?.isTest
    this.buildStatus = buildStatus(res, logAlertIndex)
    this.buildAlertCount = buildAlerts(res, logAlertIndex).length
    this.runtimeStatus = runtimeStatus(res, logAlertIndex)
    this.runtimeAlertCount = runtimeAlerts(res, logAlertIndex).length
    this.hasEndpoints = (status.endpointLinks || []).length > 0
    this.labels = getResourceLabels(res)
    this.lastBuildDur =
      lastBuild && lastBuild.startTime && lastBuild.finishTime
        ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
        : null
    this.lastDeployTime = status.lastDeployTime ?? ""
    this.pendingBuildSince = status.pendingBuildSince ?? ""
    this.currentBuildStartTime = status.currentBuild?.startTime ?? ""
    this.triggerMode = status.triggerMode ?? TriggerMode.TriggerModeAuto
    this.hasPendingChanges = !!status.hasPendingChanges
    this.queued = !!status.queued
    this.lastBuild = lastBuild
    this.hold = status.waiting ? new Hold(status.waiting) : null
    this.disabled = resourceIsDisabled(res)
  }
}

export default SidebarItem
