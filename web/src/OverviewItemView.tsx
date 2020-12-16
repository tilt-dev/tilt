import React from "react"
import { Link } from "react-router-dom"
import TimeAgo from "react-timeago"
import styled, { keyframes } from "styled-components"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { incr } from "./analytics"
import PathBuilder from "./PathBuilder"
import SidebarIcon from "./SidebarIcon"
import SidebarTriggerButton from "./SidebarTriggerButton"
import { buildStatus, runtimeStatus } from "./status"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  SizeUnit,
} from "./style-helpers"
import { formatBuildDuration, isZeroTime, timeDiff } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import { ResourceStatus, TargetType, TriggerMode } from "./types"

export const OverviewItemRoot = styled.li`
  display: flex;
  min-width: 330px;
  width: calc((100% - 3 * ${SizeUnit(0.75)} - 2 * ${SizeUnit(1)}) / 4);
  box-sizing: border-box;
  margin: 0 0 ${SizeUnit(0.75)} ${SizeUnit(0.75)};
`

type Resource = Proto.webviewResource
type Build = Proto.webviewBuildRecord

function resourceTypeLabel(res: Resource): string {
  if (res.isTiltfile) {
    return "Tiltfile"
  }
  let specs = res.specs ?? []
  for (let i = 0; i < specs.length; i++) {
    let spec = specs[i]
    if (spec.type === TargetType.K8s) {
      return "Kubernetes Deploy"
    } else if (spec.type === TargetType.DockerCompose) {
      return "Docker Compose Service"
    } else if (spec.type === TargetType.Local) {
      return "Local Script"
    }
  }
  return "Unknown"
}

export class OverviewItem {
  name: string
  resourceTypeLabel: string
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
   * Create a pared down OverviewItem from a ResourceView
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
    this.resourceTypeLabel = resourceTypeLabel(res)
  }
}

const barberpole = keyframes`
  100% {
    background-position: 100% 100%;
  }
`

export let OverviewItemBox = styled(Link)`
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  flex-direction: column;
  transition: color ${AnimDuration.default} linear,
    background-color ${AnimDuration.default} linear;
  overflow: hidden;
  border: 1px solid ${Color.grayLighter};
  position: relative; // Anchor the .isBuilding::after psuedo-element
  flex-grow: 1;
  text-decoration: none;
  font-size: ${FontSize.small};
  font-family: ${Font.monospace};
  box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.51);
  border-radius: 8px;

  &:hover {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
    color: ${Color.blue};
  }

  &.isBuilding::after {
    content: "";
    position: absolute;
    pointer-events: none;
    width: 100%;
    top: 0;
    bottom: 0;
    background: repeating-linear-gradient(
      225deg,
      ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)},
      ${ColorRGBA(Color.grayLight, ColorAlpha.translucent)} 1px,
      ${ColorRGBA(Color.black, 0)} 1px,
      ${ColorRGBA(Color.black, 0)} 6px
    );
    background-size: 200% 200%;
    animation: ${barberpole} 8s linear infinite;
  }
`

let OverviewItemRuntimeBox = styled.div`
  display: flex;
  align-items: top;
  border-bottom: 1px solid ${Color.grayLighter};
  transition: border-color ${AnimDuration.default} linear;
`

let RuntimeBoxStack = styled.div`
  display: flex;
  flex-direction: column;
  flex-grow: 1;
`

let InnerRuntimeBox = styled.div`
  display: flex;
  align-items: center;
  margin: 2px 0;
`

let OverviewItemBuildBox = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 1;
`

let OverviewItemText = styled.div`
  display: flex;
  align-items: center;
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  opacity: ${ColorAlpha.almostOpaque};
  line-height: normal;
`

let OverviewItemNameRoot = styled(OverviewItemText)`
  opacity: 1;
  font-family: ${Font.sansSerif};
  font-weight: 600;
  z-index: 1; // Appear above the .isBuilding gradient
`

let OverviewItemNameTruncate = styled.span`
  overflow: hidden;
  text-overflow: ellipsis;
`

let OverviewItemName = (props: { name: string }) => {
  // A common complaint is that long names get truncated, so we
  // use a title prop so that the user can see the full name.
  return (
    <OverviewItemNameRoot title={props.name}>
      <OverviewItemNameTruncate>{props.name}</OverviewItemNameTruncate>
    </OverviewItemNameRoot>
  )
}

let OverviewItemTimeAgo = styled.span`
  opacity: ${ColorAlpha.almostOpaque};
`

export function triggerUpdate(name: string, action: string) {
  incr("ui.web.triggerResource", { action })

  let url = `//${window.location.host}/api/trigger`

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      build_reason: 16 /* BuildReasonFlagTriggerWeb */,
    }),
  }).then((response) => {
    if (!response.ok) {
      console.log(response)
    }
  })
}

export type OverviewItemViewProps = {
  item: OverviewItem
  pathBuilder: PathBuilder
}

function buildStatusText(item: OverviewItem): string {
  let buildDur = item.lastBuildDur ? formatBuildDuration(item.lastBuildDur) : ""
  let buildStatus = item.buildStatus
  if (buildStatus === ResourceStatus.Pending) {
    return "Pending"
  } else if (buildStatus === ResourceStatus.Building) {
    return "Updating…"
  } else if (buildStatus === ResourceStatus.Unhealthy) {
    return "Update error"
  } else if (buildStatus === ResourceStatus.Healthy) {
    let msg = `Completed in ${buildDur}`
    if (item.buildAlertCount > 0) {
      msg += ", with issues"
    }
    return msg
  }
  return "Unknown"
}

function runtimeTooltipText(status: ResourceStatus): string {
  switch (status) {
    case ResourceStatus.Building:
      return "Server: deploying"
    case ResourceStatus.Pending:
      return "Server: pending"
    case ResourceStatus.Warning:
      return "Server: issues"
    case ResourceStatus.Healthy:
      return "Server: ready"
    case ResourceStatus.Unhealthy:
      return "Server: unhealthy"
    default:
      return "No server"
  }
}

function buildTooltipText(status: ResourceStatus): string {
  switch (status) {
    case ResourceStatus.Building:
      return "Update: in progress"
    case ResourceStatus.Pending:
      return "Update: pending"
    case ResourceStatus.Warning:
      return "Update: warning"
    case ResourceStatus.Healthy:
      return "Update: success"
    case ResourceStatus.Unhealthy:
      return "Update: error"
    default:
      return "No update status"
  }
}

export default function OverviewItemView(props: OverviewItemViewProps) {
  let item = props.item
  let link = `/r/${item.name}/overview`
  let formatter = timeAgoFormatter
  let hasSuccessfullyDeployed = !isZeroTime(item.lastDeployTime)
  let hasBuilt = item.lastBuild !== null
  let building = !isZeroTime(item.currentBuildStartTime)
  let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />

  let isBuildingClass = building ? "isBuilding" : ""
  let onTrigger = triggerUpdate.bind(null, item.name)

  return (
    <OverviewItemRoot key={item.name} className={`${isBuildingClass}`}>
      <OverviewItemBox
        className={`${isBuildingClass}`}
        to={props.pathBuilder.path(link)}
        data-name={item.name}
      >
        <OverviewItemRuntimeBox>
          <SidebarIcon
            tooltipText={runtimeTooltipText(item.runtimeStatus)}
            status={item.runtimeStatus}
            alertCount={item.runtimeAlertCount}
          />
          <RuntimeBoxStack style={{ margin: "8px 0px" }}>
            <InnerRuntimeBox>
              <OverviewItemText>{item.resourceTypeLabel}</OverviewItemText>
              <OverviewItemTimeAgo>
                {hasSuccessfullyDeployed ? timeAgo : "—"}
              </OverviewItemTimeAgo>
              <SidebarTriggerButton
                isTiltfile={item.isTiltfile}
                isSelected={false}
                hasPendingChanges={item.hasPendingChanges}
                hasBuilt={hasBuilt}
                isBuilding={building}
                triggerMode={item.triggerMode}
                isQueued={item.queued}
                onTrigger={onTrigger}
              />
            </InnerRuntimeBox>
            <InnerRuntimeBox>
              <OverviewItemName name={item.name} />
            </InnerRuntimeBox>
          </RuntimeBoxStack>
        </OverviewItemRuntimeBox>
        <OverviewItemBuildBox>
          <SidebarIcon
            tooltipText={buildTooltipText(item.buildStatus)}
            status={item.buildStatus}
            alertCount={item.buildAlertCount}
          />
          <OverviewItemText style={{ margin: "8px 0px" }}>
            {buildStatusText(item)}
          </OverviewItemText>
        </OverviewItemBuildBox>
      </OverviewItemBox>
    </OverviewItemRoot>
  )
}
