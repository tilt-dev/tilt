import moment from "moment"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { asUILabels, getUILabels } from "./labels"
import { LogAlertIndex } from "./LogStore"
import { buildStatus, runtimeStatus } from "./status"
import { timeDiff } from "./time"
import { ResourceName, ResourceStatus, TriggerMode } from "./types"

type UIResource = Proto.v1alpha1UIResource
type UIResourceStatus = Proto.v1alpha1UIResourceStatus
type Build = Proto.v1alpha1UIBuildTerminated

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

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: UIResource, logAlertIndex: LogAlertIndex) {
    let status = (res.status || {}) as UIResourceStatus
    let buildHistory = status.buildHistory || []
    let lastBuild = buildHistory.length > 0 ? buildHistory[0] : null
    const labels = asUILabels({ labels: res.metadata?.labels })
    this.name = res.metadata?.name ?? ""
    this.isTiltfile = this.name === ResourceName.tiltfile
    this.isTest = !!status.localResourceInfo?.isTest
    this.buildStatus = buildStatus(res, logAlertIndex)
    this.buildAlertCount = buildAlerts(res, logAlertIndex).length
    this.runtimeStatus = runtimeStatus(res, logAlertIndex)
    this.runtimeAlertCount = runtimeAlerts(res, logAlertIndex).length
    this.hasEndpoints = (status.endpointLinks || []).length > 0
    this.labels = getUILabels(labels)
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
  }
}

export default SidebarItem
