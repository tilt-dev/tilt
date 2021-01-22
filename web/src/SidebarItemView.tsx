import React from "react"
import TimeAgo from "react-timeago"
import styled, { keyframes } from "styled-components"
import { incr } from "./analytics"
import PathBuilder from "./PathBuilder"
import SidebarIcon from "./SidebarIcon"
import SidebarItem from "./SidebarItem"
import { SidebarPinButtonSpacer } from "./SidebarPin"
import SidebarPinButton from "./SidebarPinButton"
import SidebarTriggerButton from "./SidebarTriggerButton"
import {
  AnimDuration,
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  SizeUnit,
  Width,
} from "./style-helpers"
import { useTabNav } from "./TabNav"
import { formatBuildDuration, isZeroTime } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import { ResourceName, ResourceStatus, ResourceView } from "./types"

const SidebarItemRoot = styled.li`
  & + & {
    margin-top: ${SizeUnit(0.35)};
  }
  display: flex;
`

const barberpole = keyframes`
  100% {
    background-position: 100% 100%;
  }
`

export let SidebarItemBox = styled.div`
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  flex-direction: column;
  transition: color ${AnimDuration.default} linear,
    background-color ${AnimDuration.default} linear;
  border-radius: 5px;
  overflow: hidden;
  border: 1px solid ${Color.grayLighter};
  position: relative; // Anchor the .isBuilding::after psuedo-element
  flex-grow: 1;
  text-decoration: none;
  font-size: ${FontSize.small};
  font-family: ${Font.monospace};
  margin-right: ${SizeUnit(0.5)};
  cursor: pointer;

  &:hover {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
    color: ${Color.blue};
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

let SidebarItemAllBox = styled(SidebarItemBox)`
  flex-direction: row;
  height: ${SizeUnit(1.0)};
`

let SidebarItemRuntimeBox = styled.div`
  display: flex;
  align-items: center;
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
  align-items: center;
  flex-shrink: 1;
  height: ${SizeUnit(0.875)};
`

let SidebarItemAllRoot = styled(SidebarItemRoot)`
  margin-left: ${Width.sidebarPinButton}px;
  text-transform: uppercase;
`

let SidebarItemText = styled.div`
  display: flex;
  align-items: center;
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  opacity: ${ColorAlpha.almostOpaque};
  line-height: normal;
`

type SidebarItemAllProps = {
  nothingSelected: boolean
  totalAlerts: number
}

export function SidebarItemAll(props: SidebarItemAllProps) {
  let nav = useTabNav()
  return (
    <SidebarItemAllRoot>
      <SidebarItemAllBox
        className={props.nothingSelected ? "isSelected" : ""}
        tabIndex={-1}
        role="button"
        onClick={(e) =>
          nav.openResource(ResourceName.all, { newTab: e.ctrlKey || e.metaKey })
        }
      >
        <SidebarIcon
          status={ResourceStatus.None}
          alertCount={props.totalAlerts}
          tooltipText={""}
        />
        <SidebarItemNameRoot>All</SidebarItemNameRoot>
      </SidebarItemAllBox>
    </SidebarItemAllRoot>
  )
}

let SidebarItemNameRoot = styled(SidebarItemText)`
  opacity: 1;
  font-family: ${Font.sansSerif};
  font-weight: 600;
  z-index: 1; // Appear above the .isBuilding gradient
`

let SidebarItemNameTruncate = styled.span`
  overflow: hidden;
  text-overflow: ellipsis;
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

export type SidebarItemViewProps = {
  item: SidebarItem
  selected: boolean
  renderPin: boolean
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
  let nav = useTabNav()
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
  let renderPin = props.renderPin

  return (
    <SidebarItemRoot
      key={item.name}
      className={`u-showPinOnHover ${isSelectedClass} ${isBuildingClass}`}
    >
      {renderPin ? (
        <SidebarPinButton resourceName={item.name} />
      ) : (
        <SidebarPinButtonSpacer />
      )}
      <SidebarItemBox
        className={`${isSelectedClass} ${isBuildingClass}`}
        tabIndex={-1}
        role="button"
        onClick={(e) =>
          nav.openResource(item.name, { newTab: e.ctrlKey || e.metaKey })
        }
        data-name={item.name}
      >
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
        </SidebarItemBuildBox>
      </SidebarItemBox>
    </SidebarItemRoot>
  )
}
