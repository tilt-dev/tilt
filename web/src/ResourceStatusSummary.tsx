import React, { useEffect } from "react"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
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
} from "./style-helpers"
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
const ResourceGroupStatusSummaryItem = styled.li`
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
  &.is-highlightHealthy {
    color: ${Color.green};
    .fillStd {
      fill: ${Color.green};
    }
  }
`
const ResourceGroupStatusSummaryItemCount = styled.span`
  font-weight: bold;
  padding-left: 4px;
  padding-right: 4px;
`

type ResourceGroupStatusProps = {
  counts: StatusCounts
  label: string
  // TODO(matt) once we've removed OverviewResourceBar, remove this prop
  showStatusLabels: boolean
  healthyLabel: string
  unhealthyLabel: string
  warningLabel: string
}

function ResourceGroupStatus(props: ResourceGroupStatusProps) {
  if (props.counts.total === 0) {
    return null
  }
  let pb = usePathBuilder()

  let errorLink = pb.path(
    `/r/${ResourceName.all}/overview?level=${FilterLevel.error}`
  )
  let warnLink = pb.path(
    `/r/${ResourceName.all}/overview?level=${FilterLevel.warn}`
  )

  return (
    <ResourceGroupStatusRoot>
      <ResourceGroupStatusLabel>{props.label}</ResourceGroupStatusLabel>
      <ResourceGroupStatusSummaryList>
        <ResourceGroupStatusSummaryItem
          className={props.counts.unhealthy >= 1 ? "is-highlightError" : ""}
        >
          <CloseSvg width="11" title={props.unhealthyLabel}/>
          <Link to={errorLink}>
            <ResourceGroupStatusSummaryItemCount>
              {props.counts.unhealthy}
            </ResourceGroupStatusSummaryItemCount>{" "}
            {props.showStatusLabels ? props.unhealthyLabel : null}
          </Link>
        </ResourceGroupStatusSummaryItem>
        <ResourceGroupStatusSummaryItem
          className={props.counts.warning >= 1 ? "is-highlightWarning" : ""}
        >
          <WarningSvg width="7" title={props.warningLabel}/>
          <Link to={warnLink}>
            <ResourceGroupStatusSummaryItemCount>
              {props.counts.warning}
            </ResourceGroupStatusSummaryItemCount>{" "}
            {props.showStatusLabels ? props.warningLabel : null}
          </Link>
        </ResourceGroupStatusSummaryItem>
        <ResourceGroupStatusSummaryItem
          className={
            props.counts.healthy === props.counts.total
              ? "is-highlightHealthy"
              : ""
          }
        >
          <CheckmarkSmallSvg title={props.healthyLabel}/>
          <ResourceGroupStatusSummaryItemCount>
            {props.counts.healthy}
          </ResourceGroupStatusSummaryItemCount>
          /
          <ResourceGroupStatusSummaryItemCount>
            {props.counts.total}
          </ResourceGroupStatusSummaryItemCount>{" "}
          {props.showStatusLabels ? props.healthyLabel : null}
        </ResourceGroupStatusSummaryItem>
      </ResourceGroupStatusSummaryList>
    </ResourceGroupStatusRoot>
  )
}

type StatusCounts = {
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
  showStatusLabels: boolean
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
    <>
      <ResourceMetadata counts={statusCounts(resources)} />
      <ResourceGroupStatus
        counts={statusCounts(otherResources)}
        label={"Resources"}
        showStatusLabels={props.showStatusLabels}
        healthyLabel={"healthy"}
        unhealthyLabel={"err"}
        warningLabel={"warn"}
      />
      <ResourceGroupStatus
        counts={statusCounts(testResources)}
        label={"Tests"}
        showStatusLabels={props.showStatusLabels}
        healthyLabel={"pass"}
        unhealthyLabel={"fail"}
        warningLabel={"warn"}
      />
    </>
  )
}
