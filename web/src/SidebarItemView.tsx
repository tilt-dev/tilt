import React from "react"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import PathBuilder from "./PathBuilder"
import { useResourceNav } from "./ResourceNav"
import SidebarIcon from "./SidebarIcon"
import SidebarItem from "./SidebarItem"
import SidebarTriggerButton from "./SidebarTriggerButton"
import StarResourceButton, {
  StarResourceButtonRoot,
} from "./StarResourceButton"
import {
  AnimDuration,
  barberpole,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  mixinTruncateText,
  overviewItemBorderRadius,
  SizeUnit,
} from "./style-helpers"
import { formatBuildDuration, isZeroTime } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import { TriggerModeToggle } from "./TriggerModeToggle"
import { ResourceStatus, ResourceView, TriggerMode } from "./types"

export const SidebarItemRoot = styled.li`
  & + & {
    margin-top: ${SizeUnit(0.35)};
  }
  // smaller margin-left since the star icon takes up space
  margin-left: ${SizeUnit(0.25)};
  margin-right: ${SizeUnit(0.5)};
  display: flex;

  ${StarResourceButtonRoot} {
    margin-right: ${SizeUnit(1.0 / 12)};
  }
`

export let SidebarItemBox = styled.div`
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  flex-grow: 1;
  transition: color ${AnimDuration.default} linear,
    background-color ${AnimDuration.default} linear;
  border-radius: ${overviewItemBorderRadius};
  overflow: hidden;
  border: 1px solid ${Color.grayLighter};
  position: relative; // Anchor the .isBuilding::after psuedo-element
  text-decoration: none;
  font-size: ${FontSize.small};
  font-family: ${Font.monospace};
  cursor: pointer;

  &:hover {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }

  &.isSelected {
    background-color: ${Color.white};
    color: ${Color.gray};
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

// Flexbox (column) containing:
// - `SidebarItemRuntimeBox` - (row) with runtime status, name, star, timeago
// - `SidebarItemBuildBox` - (row) with build status, text
let SidebarItemInnerBox = styled.div`
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  // To truncate long resource names…
  min-width: 0; // Override default, so width can be less than content
`

let SidebarItemRuntimeBox = styled.div`
  display: flex;
  flex-grow: 1;
  align-items: stretch;
  height: ${SizeUnit(1)};
  border-bottom: 1px solid ${Color.grayLighter};
  box-sizing: border-box;
  transition: border-color ${AnimDuration.default} linear;

  .isSelected & {
    border-bottom-color: ${Color.grayLightest};
  }
`

let SidebarItemBuildBox = styled.div`
  display: flex;
  align-items: stretch;
  padding-right: 4px;
`
let SidebarItemText = styled.div`
  display: flex;
  align-items: center;
  flex-grow: 1;
  padding-top: 4px;
  padding-bottom: 4px;
  color: ${Color.grayLightest};
`

let SidebarItemNameRoot = styled.div`
  display: flex;
  align-items: center;
  font-family: ${Font.sansSerif};
  font-weight: 600;
  z-index: 1; // Appear above the .isBuilding gradient
  // To truncate long resource names…
  min-width: 0; // Override default, so width can be less than content
`
let SidebarItemNameTruncate = styled.span`
  ${mixinTruncateText}
`

let SidebarItemName = (props: { name: string }) => {
  // A common complaint is that long names get truncated, so we
  // use a title prop so that the user can see the full name.
  return (
    <SidebarItemNameRoot title={props.name}>
      <SidebarItemNameTruncate>{props.name}</SidebarItemNameTruncate>
    </SidebarItemNameRoot>
  )
}

let SidebarItemTimeAgo = styled.span`
  opacity: ${ColorAlpha.almostOpaque};
  display: flex;
  justify-content: flex-end;
  flex-grow: 1;
  align-items: center;
  text-align: right;
  white-space: nowrap;
  padding-right: ${SizeUnit(0.25)};
`

export function triggerUpdate(name: string) {
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

export function toggleTriggerMode(name: string, mode: TriggerMode) {
  let url = "/api/override/trigger_mode"

  fetch(url, {
    method: "post",
    body: JSON.stringify({
      manifest_names: [name],
      trigger_mode: mode,
    }),
  }).then((response) => {
    if (!response.ok) {
      console.log(response)
    }
  })
}

export type SidebarItemViewProps = {
  item: SidebarItem
  selected: boolean
  resourceView: ResourceView
  pathBuilder: PathBuilder
}

function buildStatusText(item: SidebarItem): string {
  let buildDur = item.lastBuildDur ? formatBuildDuration(item.lastBuildDur) : ""
  let buildStatus = item.buildStatus
  if (buildStatus === ResourceStatus.Pending) {
    return "Pending"
  } else if (buildStatus === ResourceStatus.Building) {
    return "Updating…"
  } else if (buildStatus === ResourceStatus.None) {
    return "No update status"
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

export default function SidebarItemView(props: SidebarItemViewProps) {
  let nav = useResourceNav()
  let item = props.item
  let formatter = timeAgoFormatter
  let hasSuccessfullyDeployed = !isZeroTime(item.lastDeployTime)
  let hasBuilt = item.lastBuild !== null
  let building = !isZeroTime(item.currentBuildStartTime)
  let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />
  let isSelected = props.selected

  let isSelectedClass = isSelected ? "isSelected" : ""
  let isBuildingClass = building ? "isBuilding" : ""
  let onTrigger = triggerUpdate.bind(null, item.name)
  let onModeToggle = toggleTriggerMode.bind(null, item.name)

  return (
    <SidebarItemRoot
      key={item.name}
      className={`u-showStarOnHover u-showTriggerModeOnHover ${isSelectedClass} ${isBuildingClass}`}
    >
      <StarResourceButton
        resourceName={item.name}
        analyticsName="ui.web.sidebarStarButton"
      />
      <SidebarItemBox
        className={`${isSelectedClass} ${isBuildingClass}`}
        tabIndex={-1}
        role="button"
        onClick={(e) => nav.openResource(item.name)}
        data-name={item.name}
      >
        <SidebarItemInnerBox>
          <SidebarItemRuntimeBox>
            <SidebarIcon
              tooltipText={runtimeTooltipText(item.runtimeStatus)}
              status={item.runtimeStatus}
              alertCount={item.runtimeAlertCount}
            />
            <SidebarItemName name={item.name} />
            <SidebarItemTimeAgo>
              {hasSuccessfullyDeployed ? timeAgo : "—"}
            </SidebarItemTimeAgo>
            <SidebarTriggerButton
              isTiltfile={item.isTiltfile}
              isSelected={isSelected}
              hasPendingChanges={item.hasPendingChanges}
              hasBuilt={hasBuilt}
              isBuilding={building}
              triggerMode={item.triggerMode}
              isQueued={item.queued}
              onTrigger={onTrigger}
            />
          </SidebarItemRuntimeBox>
          <SidebarItemBuildBox>
            <SidebarIcon
              tooltipText={buildTooltipText(item.buildStatus)}
              status={item.buildStatus}
              alertCount={item.buildAlertCount}
            />
            <SidebarItemText>{buildStatusText(item)}</SidebarItemText>
            {item.isTest && (
              <TriggerModeToggle
                triggerMode={item.triggerMode}
                onModeToggle={onModeToggle}
              />
            )}
          </SidebarItemBuildBox>
        </SidebarItemInnerBox>
      </SidebarItemBox>
    </SidebarItemRoot>
  )
}
