import React, { useEffect } from "react"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as PendingSvg } from "./assets/svg/pending.svg"
import { ReactComponent as WarningSvg } from "./assets/svg/warning.svg"
import { FilterLevel } from "./logfilters"
import { usePathBuilder } from "./PathBuilder"
import { combinedStatus } from "./status"
import {
  Color,
  Font,
  FontSize,
  mixinResetListStyle,
  SizeUnit,
  spin,
} from "./style-helpers"
import Tooltip from "./Tooltip"
import { ResourceName, ResourceStatus } from "./types"

const ResourceGroupStatusRoot = styled.div`
  display: flex;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallest};
  align-items: center;
  color: ${Color.grayLightest};

  .fillStd {
    fill: ${Color.grayLighter};
  }

  & + & {
    margin-left: ${SizeUnit(1.5)};
  }
`

const ResourceGroupStatusLabel = styled.p`
  text-transform: uppercase;
  margin-right: ${SizeUnit(0.5)};
`
const ResourceGroupStatusSummaryList = styled.ul`
  display: flex;
  ${mixinResetListStyle}
`
const ResourceGroupStatusSummaryItemRoot = styled.div`
  display: flex;
  align-items: center;

  & + & {
    margin-left: ${SizeUnit(0.25)};
    border-left: 1px solid ${Color.grayLighter};
    padding-left: ${SizeUnit(0.25)};
  }
  &.is-highlightError {
    color: ${Color.red};
    .fillStd {
      fill: ${Color.red};
    }
  }
  &.is-highlightWarning {
    color: ${Color.yellow};
    .fillStd {
      fill: ${Color.yellow};
    }
  }
  &.is-highlightPending {
    color: ${Color.gray7};
    stroke: ${Color.gray7};
    .fillStd {
      fill: ${Color.gray7};
    }
  }
  &.is-highlightHealthy {
    color: ${Color.green};
    .fillStd {
      fill: ${Color.green};
    }
  }
`
export const ResourceGroupStatusSummaryItemCount = styled.div`
  font-weight: bold;
  padding-left: 4px;
  padding-right: 4px;
`
export const ResourceStatusSummaryRoot = styled.div`
  display: flex;
`
export const PendingIcon = styled(PendingSvg)`
  animation: ${spin} 4s linear infinite;
`

type ResourceGroupStatusItemProps = {
  label: string
  icon: JSX.Element
  className: string
  count: number
  countOutOf?: number
  href?: string
}
export function ResourceGroupStatusItem(props: ResourceGroupStatusItemProps) {
  const count = (
    <>
      <ResourceGroupStatusSummaryItemCount>
        {props.count}
      </ResourceGroupStatusSummaryItemCount>
      {props.countOutOf && (
        <>
          /
          <ResourceGroupStatusSummaryItemCount>
            {props.countOutOf}
          </ResourceGroupStatusSummaryItemCount>
        </>
      )}
    </>
  )

  let inner = count
  if (props.href) {
    inner = <Link to={props.href}>{count}</Link>
  }

  return (
    <Tooltip title={props.label}>
      <ResourceGroupStatusSummaryItemRoot className={props.className}>
        {props.icon}
        {inner}
      </ResourceGroupStatusSummaryItemRoot>
    </Tooltip>
  )
}

type ResourceGroupStatusProps = {
  counts: StatusCounts
  label: string
  healthyLabel: string
  unhealthyLabel: string
  warningLabel: string
}

export function ResourceGroupStatus(props: ResourceGroupStatusProps) {
  if (props.counts.total === 0) {
    return null
  }
  let pb = usePathBuilder()

  let items = new Array<JSX.Element>()

  if (props.counts.unhealthy) {
    items.push(
      <ResourceGroupStatusItem
        key={props.unhealthyLabel}
        label={props.unhealthyLabel}
        count={props.counts.unhealthy}
        href={pb.encpath`/r/${ResourceName.all}/overview?level=${FilterLevel.error}`}
        className="is-highlightError"
        icon={<CloseSvg width="11" key="icon" />}
      />
    )
  }

  if (props.counts.warning) {
    items.push(
      <ResourceGroupStatusItem
        key={props.warningLabel}
        label={props.warningLabel}
        count={props.counts.warning}
        href={pb.encpath`/r/${ResourceName.all}/overview?level=${FilterLevel.warn}`}
        className="is-highlightWarning"
        icon={<WarningSvg width="7" key="icon" />}
      />
    )
  }

  if (props.counts.pending) {
    items.push(
      <ResourceGroupStatusItem
        key="pending"
        label="pending"
        count={props.counts.pending}
        className="is-highlightPending"
        icon={<PendingIcon width="8" key="icon" />}
      />
    )
  }

  // always show healthy count
  items.push(
    <ResourceGroupStatusItem
      key={props.healthyLabel}
      label={props.healthyLabel}
      count={props.counts.healthy}
      countOutOf={props.counts.total}
      className="is-highlightHealthy"
      icon={<CheckmarkSmallSvg key="icon" />}
    />
  )

  return (
    <ResourceGroupStatusRoot>
      <ResourceGroupStatusLabel>{props.label}</ResourceGroupStatusLabel>
      <ResourceGroupStatusSummaryList>{items}</ResourceGroupStatusSummaryList>
    </ResourceGroupStatusRoot>
  )
}

export type StatusCounts = {
  total: number
  healthy: number
  unhealthy: number
  pending: number
  warning: number
}

function statusCounts(resources: Proto.webviewResource[]): StatusCounts {
  let statuses = resources.map((res) => combinedStatus(res))
  let allStatusCount = 0
  let healthyStatusCount = 0
  let unhealthyStatusCount = 0
  let pendingStatusCount = 0
  let warningCount = 0
  statuses.forEach((status) => {
    switch (status) {
      case ResourceStatus.Warning:
        allStatusCount++
        healthyStatusCount++
        warningCount++
        break
      case ResourceStatus.Healthy:
        allStatusCount++
        healthyStatusCount++
        break
      case ResourceStatus.Unhealthy:
        allStatusCount++
        unhealthyStatusCount++
        break
      case ResourceStatus.Pending:
      case ResourceStatus.Building:
        allStatusCount++
        pendingStatusCount++
        break
      default:
      // Don't count None status in the overall resource count.
      // These might be manual tasks we haven't run yet.
    }
  })

  return {
    total: allStatusCount,
    healthy: healthyStatusCount,
    unhealthy: unhealthyStatusCount,
    pending: pendingStatusCount,
    warning: warningCount,
  }
}

function ResourceMetadata(props: { counts: StatusCounts }) {
  let { total, healthy, pending, unhealthy } = props.counts
  useEffect(() => {
    let favicon: any = document.head.querySelector("#favicon")
    let faviconHref = ""
    if (unhealthy > 0) {
      document.title = `✖︎ ${unhealthy} ┊ Tilt`
      faviconHref = "/static/ico/favicon-red.ico"
    } else if (pending || total === 0) {
      document.title = `… ${healthy}/${total} ┊ Tilt`
      faviconHref = "/static/ico/favicon-gray.ico"
    } else {
      document.title = `✔︎ ${healthy}/${total} ┊ Tilt`
      faviconHref = "/static/ico/favicon-green.ico"
    }
    if (favicon) {
      favicon.href = faviconHref
    }
  }, [total, healthy, pending, unhealthy])
  return <></>
}

type ResourceStatusSummaryProps = {
  view: Proto.webviewView
}

export function ResourceStatusSummary(props: ResourceStatusSummaryProps) {
  // Count the statuses.
  let resources = props.view.resources || []

  let testResources = new Array<Proto.webviewResource>()
  let otherResources = new Array<Proto.webviewResource>()
  resources.forEach((r) => {
    if (r.localResourceInfo && r.localResourceInfo.isTest) {
      testResources.push(r)
    } else {
      otherResources.push(r)
    }
  })

  return (
    <ResourceStatusSummaryRoot>
      <ResourceMetadata counts={statusCounts(resources)} />
      <ResourceGroupStatus
        counts={statusCounts(otherResources)}
        label={"Resources"}
        healthyLabel={"healthy"}
        unhealthyLabel={"err"}
        warningLabel={"warn"}
      />
      <ResourceGroupStatus
        counts={statusCounts(testResources)}
        label={"Tests"}
        healthyLabel={"pass"}
        unhealthyLabel={"fail"}
        warningLabel={"warn"}
      />
    </ResourceStatusSummaryRoot>
  )
}
