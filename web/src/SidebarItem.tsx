import styled from "styled-components"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { buildStatus, runtimeStatus } from "./status"
import { SizeUnit } from "./style-helpers"
import { ResourceStatus, TriggerMode } from "./types"

export const SidebarItemRoot = styled.li`
  & + & {
    margin-top: ${SizeUnit(0.2)};
  }
  clear: both;
  display: flex;
`

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
  buildStatus: ResourceStatus
  buildAlertCount: number
  runtimeStatus: ResourceStatus
  runtimeAlertCount: number
  hasEndpoints: boolean
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
  constructor(res: Resource) {
    let buildHistory = res.buildHistory || []
    let lastBuild = buildHistory.length > 0 ? buildHistory[0] : null

    this.name = res.name ?? ""
    this.isTiltfile = !!res.isTiltfile
    this.buildStatus = buildStatus(res)
    this.buildAlertCount = buildAlerts(res, null).length
    this.runtimeStatus = runtimeStatus(res)
    this.runtimeAlertCount = runtimeAlerts(res, null).length
    this.hasEndpoints = (res.endpointLinks || []).length > 0
    this.lastBuildDur =
      lastBuild && lastBuild.startTime && lastBuild.finishTime
        ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
        : null
    this.lastDeployTime = res.lastDeployTime ?? ""
    this.pendingBuildSince = res.pendingBuildSince ?? ""
    this.currentBuildStartTime = res.currentBuild?.startTime ?? ""
    this.triggerMode = res.triggerMode ?? TriggerMode.TriggerModeAuto
    this.hasPendingChanges = !!res.hasPendingChanges
    this.queued = !!res.queued
    this.lastBuild = lastBuild
  }
}

export default SidebarItem
