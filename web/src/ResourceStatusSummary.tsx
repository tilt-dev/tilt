import React, { useEffect } from "react"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as DisabledSvg } from "./assets/svg/not-allowed.svg"
import { ReactComponent as PendingSvg } from "./assets/svg/pending.svg"
import { ReactComponent as WarningSvg } from "./assets/svg/warning.svg"
import { FilterLevel } from "./logfilters"
import { useLogStore } from "./LogStore"
import { RowValues } from "./OverviewTableColumns"
import { usePathBuilder } from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SrOnly from "./SrOnly"
import { buildStatus, combinedStatus, runtimeStatus } from "./status"
import {
  Color,
  Font,
  FontSize,
  mixinResetListStyle,
  SizeUnit,
  spin,
} from "./style-helpers"
import Tooltip from "./Tooltip"
import { ResourceName, ResourceStatus, UIResource } from "./types"

const ResourceGroupStatusLabel = styled.p`
  text-transform: uppercase;
  margin-right: ${SizeUnit(0.5)};
`
const ResourceGroupStatusSummaryList = styled.ul`
  display: flex;
  ${mixinResetListStyle}
`
const ResourceGroupStatusSummaryItemRoot = styled.li`
  display: flex;
  align-items: center;

  & + & {
    margin-left: ${SizeUnit(0.25)};
    border-left: 1px solid ${Color.gray40};
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
    color: ${Color.gray70};
    stroke: ${Color.gray70};
    .fillStd {
      fill: ${Color.gray70};
    }
  }
  &.is-highlightHealthy {
    color: ${Color.green};
    .fillStd {
      fill: ${Color.green};
    }
  }
`
export const ResourceGroupStatusSummaryItemCount = styled.span`
  font-weight: bold;
  padding-left: 4px;
  padding-right: 4px;
`
export const ResourceStatusSummaryRoot = styled.aside`
  display: flex;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallest};
  align-items: center;
  color: ${Color.grayLightest};

  .fillStd {
    fill: ${Color.gray40};
  }

  & + & {
    margin-left: ${SizeUnit(1.5)};
  }
`
export const PendingIcon = styled(PendingSvg)`
  animation: ${spin} 4s linear infinite;
`

const DisabledIcon = styled(DisabledSvg)`
  .fillStd {
    fill: ${Color.gray60};
  }
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

  const descriptiveCount = (
    <>
      {count} <SrOnly>&nbsp; {props.label}</SrOnly>
    </>
  )
  const summaryContent = props.href ? (
    <Link to={props.href}>{descriptiveCount}</Link>
  ) : (
    descriptiveCount
  )

  return (
    <Tooltip title={props.label}>
      <ResourceGroupStatusSummaryItemRoot className={props.className}>
        {props.icon}
        {summaryContent}
      </ResourceGroupStatusSummaryItemRoot>
    </Tooltip>
  )
}

type ResourceGroupStatusProps = {
  counts: StatusCounts
  displayText?: string
  labelText: string // Used for a11y markup, should be a descriptive title.
  healthyLabel: string
  unhealthyLabel: string
  warningLabel: string
  linkToLogFilters: boolean
}

export function ResourceGroupStatus(props: ResourceGroupStatusProps) {
  if (props.counts.totalEnabled === 0 && props.counts.disabled === 0) {
    return null
  }
  let pb = usePathBuilder()

  let items = new Array<JSX.Element>()

  if (props.counts.unhealthy) {
    const errorHref = props.linkToLogFilters
      ? pb.encpath`/r/${ResourceName.all}/overview?level=${FilterLevel.error}`
      : undefined
    items.push(
      <ResourceGroupStatusItem
        key={props.unhealthyLabel}
        label={props.unhealthyLabel}
        count={props.counts.unhealthy}
        href={errorHref}
        className="is-highlightError"
        icon={<CloseSvg role="presentation" width="11" key="icon" />}
      />
    )
  }

  if (props.counts.warning) {
    const warningHref = props.linkToLogFilters
      ? pb.encpath`/r/${ResourceName.all}/overview?level=${FilterLevel.warn}`
      : undefined
    items.push(
      <ResourceGroupStatusItem
        key={props.warningLabel}
        label={props.warningLabel}
        count={props.counts.warning}
        href={warningHref}
        className="is-highlightWarning"
        icon={<WarningSvg role="presentation" width="7" key="icon" />}
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
        icon={<PendingIcon role="presentation" width="8" key="icon" />}
      />
    )
  }

  // There might not always be enabled resources
  // if all resources are disabled
  if (props.counts.totalEnabled) {
    items.push(
      <ResourceGroupStatusItem
        key={props.healthyLabel}
        label={props.healthyLabel}
        count={props.counts.healthy}
        countOutOf={props.counts.totalEnabled}
        className="is-highlightHealthy"
        icon={<CheckmarkSmallSvg role="presentation" key="icon" />}
      />
    )
  }

  if (props.counts.disabled) {
    items.push(
      <ResourceGroupStatusItem
        key="disabled"
        label="disabled"
        count={props.counts.disabled}
        className="is-highlightDisabled"
        icon={<DisabledIcon role="presentation" width="15" key="icon" />}
      />
    )
  }

  const displayLabel = props.displayText ? (
    <ResourceGroupStatusLabel>{props.displayText}</ResourceGroupStatusLabel>
  ) : null

  return (
    <>
      {displayLabel}
      <ResourceGroupStatusSummaryList>{items}</ResourceGroupStatusSummaryList>
    </>
  )
}

export type StatusCounts = {
  totalEnabled: number
  healthy: number
  unhealthy: number
  pending: number
  warning: number
  disabled: number
}

function statusCounts(statuses: ResourceStatus[]): StatusCounts {
  let allEnabledStatusCount = 0
  let healthyStatusCount = 0
  let unhealthyStatusCount = 0
  let pendingStatusCount = 0
  let warningCount = 0
  let disabledCount = 0
  statuses.forEach((status) => {
    switch (status) {
      case ResourceStatus.Warning:
        allEnabledStatusCount++
        healthyStatusCount++
        warningCount++
        break
      case ResourceStatus.Healthy:
        allEnabledStatusCount++
        healthyStatusCount++
        break
      case ResourceStatus.Unhealthy:
        allEnabledStatusCount++
        unhealthyStatusCount++
        break
      case ResourceStatus.Pending:
      case ResourceStatus.Building:
        allEnabledStatusCount++
        pendingStatusCount++
        break
      case ResourceStatus.Disabled:
        disabledCount++
        break
      default:
      // Don't count None status in the overall resource count.
      // These might be manual tasks we haven't run yet.
    }
  })

  return {
    totalEnabled: allEnabledStatusCount,
    healthy: healthyStatusCount,
    unhealthy: unhealthyStatusCount,
    pending: pendingStatusCount,
    warning: warningCount,
    disabled: disabledCount,
  }
}

function ResourceMetadata(props: { counts: StatusCounts }) {
  let { totalEnabled, healthy, pending, unhealthy } = props.counts
  useEffect(() => {
    let favicon: any = document.head.querySelector("#favicon")
    let faviconHref = ""
    if (unhealthy > 0) {
      document.title = `✖︎ ${unhealthy} ┊ Tilt`
      faviconHref = "/static/ico/favicon-red.ico"
    } else if (pending || totalEnabled === 0) {
      document.title = `… ${healthy}/${totalEnabled} ┊ Tilt`
      faviconHref = "/static/ico/favicon-gray.ico"
    } else {
      document.title = `✔︎ ${healthy}/${totalEnabled} ┊ Tilt`
      faviconHref = "/static/ico/favicon-green.ico"
    }
    if (favicon) {
      favicon.href = faviconHref
    }
  }, [totalEnabled, healthy, pending, unhealthy])
  return <></>
}

type ResourceStatusSummaryOptions = {
  displayText?: string
  labelText?: string
  updateMetadata?: boolean
  linkToLogFilters?: boolean
}

type ResourceStatusSummaryProps = {
  statuses: ResourceStatus[]
} & ResourceStatusSummaryOptions

function ResourceStatusSummary(props: ResourceStatusSummaryProps) {
  // Default the display options if no option is provided
  const updateMetadata = props.updateMetadata ?? true
  const linkToLogFilters = props.linkToLogFilters ?? true
  const labelText = props.labelText ?? "Resource status summary"

  return (
    <ResourceStatusSummaryRoot aria-label={labelText}>
      {updateMetadata && (
        <ResourceMetadata counts={statusCounts(props.statuses)} />
      )}
      <ResourceGroupStatus
        counts={statusCounts(props.statuses)}
        displayText={props.displayText}
        labelText={labelText}
        healthyLabel={"healthy"}
        unhealthyLabel={"err"}
        warningLabel={"warn"}
        linkToLogFilters={linkToLogFilters}
      />
    </ResourceStatusSummaryRoot>
  )
}

// The generic StatusSummaryProps takes a template type
// for the resources it will summarize, so that it can be used
// throughout the app with different data types.

type StatusSummaryProps<T> = {
  resources: readonly T[]
} & ResourceStatusSummaryOptions

export function SidebarGroupStatusSummary(
  props: StatusSummaryProps<SidebarItem>
) {
  const allStatuses = props.resources.map((item: SidebarItem) =>
    combinedStatus(item.buildStatus, item.runtimeStatus)
  )

  return (
    <ResourceStatusSummary
      statuses={allStatuses}
      linkToLogFilters={false}
      updateMetadata={false}
      {...props}
    />
  )
}

export function TableGroupStatusSummary(props: StatusSummaryProps<RowValues>) {
  const allStatuses = props.resources.map((r: RowValues) =>
    combinedStatus(r.statusLine.buildStatus, r.statusLine.runtimeStatus)
  )

  return (
    <ResourceStatusSummary
      statuses={allStatuses}
      linkToLogFilters={false}
      updateMetadata={false}
      {...props}
    />
  )
}

export function AllResourceStatusSummary(
  props: StatusSummaryProps<UIResource>
) {
  const logStore = useLogStore()
  const allStatuses = props.resources.map((r: UIResource) =>
    combinedStatus(buildStatus(r, logStore), runtimeStatus(r, logStore))
  )

  return <ResourceStatusSummary statuses={allStatuses} {...props} />
}
