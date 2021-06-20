import React from "react"
import { CellProps, Column, useSortBy, useTable } from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { buildAlerts } from "./alerts"
import { incr } from "./analytics"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { ReactComponent as PendingSvg } from "./assets/svg/pending.svg"
import { displayURL } from "./links"
import { useResourceNav } from "./ResourceNav"
import { useStarredResources } from "./StarredResourcesContext"
import { buildStatus, runtimeStatus } from "./status"
import { Color, Font, FontSize, Glow, SizeUnit, spin } from "./style-helpers"
import TableStarResourceButton from "./TableStarResourceButton"
import { formatBuildDuration, isZeroTime, timeDiff } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import TriggerButton from "./TriggerButton"
import { TriggerModeToggle } from "./TriggerModeToggle"
import { ResourceStatus, TargetType, TriggerMode } from "./types"

type UIResource = Proto.v1alpha1UIResource
type UIResourceStatus = Proto.v1alpha1UIResourceStatus
type Build = Proto.v1alpha1UIBuildTerminated
type UILink = Proto.v1alpha1UIResourceLink

type OverviewTableProps = {
  view: Proto.webviewView
}

type RowValues = {
  // lastBuild: Build | null
  statusText: StatusTextType
  currentBuildStartTime: string
  endpoints: UILink[]
  hasPendingChanges: boolean
  lastDeployTime: string
  name: string
  podId: string
  queued: boolean
  resourceTypeLabel: string
  triggerMode: TriggerMode
}

type StatusTextType = {
  buildStatus: ResourceStatus
  buildAlertCount: number
  lastBuildDur: moment.Duration | null
  runtimeStatus: ResourceStatus
}

const ResourceTable = styled.table`
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(1)};
  margin-right: ${SizeUnit(1)};
  border-collapse: collapse;
`
const ResourceTableHead = styled.thead`
  background-color: ${Color.grayDarker};
`
const ResourceTableRow = styled.tr`
  border-bottom: 1px solid ${Color.grayLighter};
`
const ResourceTableData = styled.td`
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
  color: ${Color.gray6};
  padding-top: ${SizeUnit(0.25)};
  padding-bottom: ${SizeUnit(0.25)};
  box-sizing: border-box;
`
const ResourceTableHeader = styled(ResourceTableData)`
  color: ${Color.gray7};
  font-size: ${FontSize.smallester};
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  box-sizing: border-box;
`

const ResourceTableHeaderSort = styled.span`
  opacity: 0;
  &.is-sorted-asc {
    opacity: 1;
  }
  &.is-sorted-desc {
    opacity: 1;
  }
`
const ResourceTableHeaderSortTriangle = styled.span`
  width: 0;
  height: 0;
  border-left: 20px solid transparent;
  border-right: 20px solid transparent;
  border-top: 20px solid ${Color.gray};
`
const ResourceName = styled.div`
  color: ${Color.offWhite};
  font-size: ${FontSize.small};
  &.has-error {
    color: ${Color.red};
  }
`

const Endpoint = styled.a``
const StyledLinkSvg = styled(LinkSvg)``

const DetailText = styled.span`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-left: 10px;
`

const StyledStatus = styled.div``

const StatusMsg = styled.span``
const StatusIcon = styled.span`
  display: flex;
  align-items: center;
  margin-right: ${SizeUnit(0.2)};
  width: ${SizeUnit(0.5)};

  svg {
    width: 100%;
  }
`
const BuildStatusIcon = styled(StatusIcon)``
const RuntimeStatusIcon = styled(StatusIcon)``
const BuildStatusMsg = styled.span``
const RuntimeStatusMsg = styled.span`
  ${BuildStatusMsg} + & {
    padding-left: ${SizeUnit(0.25)};
    border-left: 1px solid ${Color.gray};
    margin-left: ${SizeUnit(0.25)};
  }
`
const StatusLine = styled.div`
  display: flex;
  align-items: center;

  &.is-healthy {
    svg {
      fill: ${Color.green};
    }
  }
  &.is-building,
  &.is-pending {
    svg {
      fill: ${Color.grayLightest};
      animation: ${spin} 4s linear infinite;
      width: 80%;
    }
  }
  &.is-pending {
    ${StatusMsg} {
      animation: ${Glow.opacity} 2s linear infinite;
    }
  }
  &.is-error {
    color: ${Color.red};
    svg {
      fill: ${Color.red};
    }
  }
  &.is-none {
    color: ${Color.grayLight};
  }
`

function columnDefs(): Column<RowValues>[] {
  let ctx = useStarredResources()

  return React.useMemo(
    () => [
      {
        Header: "Starred",
        width: "20px",
        Cell: ({ row }: CellProps<RowValues>) => {
          return (
            <TableStarResourceButton
              resourceName={row.values.name}
              analyticsName="ui.web.overviewStarButton"
              ctx={ctx}
            />
          )
        },
      },
      {
        Header: "Last Updated",
        accessor: "lastDeployTime",
        Cell: ({ row }: CellProps<RowValues>) => {
          return (
            <TimeAgo
              date={row.values.lastDeployTime}
              formatter={timeAgoFormatter}
            />
          )
        },
      },
      {
        Header: "Trigger",
        width: "20px",
        Cell: ({ row }: CellProps<RowValues>) => {
          let building = !isZeroTime(row.values.currentBuildStartTime)
          let hasBuilt = row.values.lastBuild !== null
          return (
            <TriggerButton
              hasPendingChanges={row.values.hasPendingChanges}
              hasBuilt={hasBuilt}
              isBuilding={building}
              triggerMode={row.values.triggerMode}
              isQueued={row.values.queued}
              resourceName={row.values.name}
            />
          )
        },
      },
      {
        Header: "Resource Name",
        width: "200px",
        accessor: "name",
        Cell: ({ row }: CellProps<RowValues>) => {
          let nav = useResourceNav()
          let hasError =
            row.values.statusText.buildStatus === ResourceStatus.Unhealthy ||
            row.values.statusText.runtimeStatus === ResourceStatus.Unhealthy

          return (
            <ResourceName
              className={hasError ? "has-error" : ""}
              onClick={(e) => nav.openResource(row.values.name)}
            >
              {row.values.name}
            </ResourceName>
          )
        },
      },
      {
        Header: "Type",
        accessor: "resourceTypeLabel",
      },
      {
        Header: "Status",
        accessor: "statusText",
        Cell: ({ row }: CellProps<RowValues>) => {
          return statusText(
            row.values.statusText.buildStatus,
            row.values.statusText.lastBuildDur,
            row.values.statusText.buildAlertCount,
            row.values.statusText.runtimeStatus
          )
        },
      },
      {
        Header: "Pod ID",
        accessor: "podId",
      },
      {
        Header: "Endpoints",
        accessor: "endpoints",
        Cell: ({ row }: CellProps<RowValues>) => {
          // @ts-ignore
          let endpoints = row.values.endpoints.map((ep) => {
            return (
              <Endpoint
                onClick={() =>
                  void incr("ui.web.endpoint", { action: "click" })
                }
                href={ep.url}
                // We use ep.url as the target, so that clicking the link re-uses the tab.
                target={ep.url}
                key={ep.url}
              >
                <StyledLinkSvg />
                <DetailText>{ep.name || displayURL(ep)}</DetailText>
              </Endpoint>
            )
          })
          return <div>{endpoints}</div>
        },
      },
      {
        Header: "Trigger Mode",
        width: 1,
        Cell: ({ row }: CellProps<RowValues>) => {
          let onModeToggle = toggleTriggerMode.bind(null, row.values.name)
          return (
            <TriggerModeToggle
              triggerMode={row.values.triggerMode}
              onModeToggle={onModeToggle}
            />
          )
        },
      },
    ],
    [ctx.starredResources]
  )
}

function uiResourceToCell(r: UIResource): RowValues {
  let res = (r.status || {}) as UIResourceStatus
  let buildHistory = res.buildHistory || []
  let lastBuild: Build | null = buildHistory.length > 0 ? buildHistory[0] : null

  return {
    statusText: {
      buildStatus: buildStatus(r),
      lastBuildDur:
        lastBuild && lastBuild.startTime && lastBuild.finishTime
          ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
          : null,
      buildAlertCount: buildAlerts(r, null).length,
      runtimeStatus: runtimeStatus(r),
    },
    currentBuildStartTime: res.currentBuild?.startTime ?? "",
    endpoints: res.endpointLinks ?? [],
    hasPendingChanges: !!res.hasPendingChanges,
    lastDeployTime: res.lastDeployTime ?? "",
    name: r.metadata?.name ?? "",
    podId: res.k8sResourceInfo?.podName ?? "",
    queued: !!res.queued,
    resourceTypeLabel: resourceTypeLabel(r),
    triggerMode: res.triggerMode ?? TriggerMode.TriggerModeAuto,
  }
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

function resourceTypeLabel(r: UIResource): string {
  let res = (r.status || {}) as UIResourceStatus
  let name = r.metadata?.name
  if (name == "(Tiltfile)") {
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
      if (res.localResourceInfo && !!res.localResourceInfo.isTest) {
        return "Test"
      }
      return "Local Script"
    }
  }
  return "Unknown"
}

function statusText(
  buildStatus: ResourceStatus,
  lastBuildDur: moment.Duration | null,
  buildAlertCount: number,
  runtimeStatus: ResourceStatus
) {
  let buildIcon = null
  let buildMsg = ""
  let runtimeIcon = null
  let runtimeMsg = ""
  let buildClass = ""
  let runtimeClass = ""

  switch (buildStatus) {
    case ResourceStatus.Building:
      buildIcon = <PendingSvg />
      buildMsg = "Updating…"
      buildClass = "is-building"
      break
    case ResourceStatus.None:
      break
    case ResourceStatus.Pending:
      buildIcon = <PendingSvg />
      buildMsg = "Update Pending"
      buildClass = "is-pending"
      break
    case ResourceStatus.Warning:
      break
    case ResourceStatus.Healthy:
      let buildDurText = lastBuildDur
        ? ` in ${formatBuildDuration(lastBuildDur)}`
        : ""
      buildIcon = <CheckmarkSmallSvg />
      buildMsg = `Updated${buildDurText}`
      buildClass = "is-healthy"

      if (buildAlertCount > 0) {
        buildMsg += ", with issues"
      }
      break
    case ResourceStatus.Unhealthy:
      buildIcon = <CloseSvg />
      buildMsg = "Update error"
      buildClass = "is-error"
      break
    default:
      buildMsg = ""
  }

  switch (runtimeStatus) {
    case ResourceStatus.Building:
      runtimeIcon = <PendingSvg />
      runtimeMsg = "Runtime deploying"
      runtimeClass = "is-building"
      break
    case ResourceStatus.Pending:
      runtimeIcon = <PendingSvg />
      runtimeMsg = "Runtime pending"
      runtimeClass = "is-pending"
      break
    case ResourceStatus.Warning:
      runtimeMsg = "Runtime issues"
      break
    case ResourceStatus.Healthy:
      runtimeIcon = <CheckmarkSmallSvg />
      runtimeMsg = "Runtime ready"
      runtimeClass = "is-healthy"
      break
    case ResourceStatus.Unhealthy:
      runtimeIcon = <CloseSvg />
      runtimeMsg = "Runtime error"
      runtimeClass = "is-error"
      break
    default:
      runtimeMsg = ""
  }

  return (
    <StyledStatus>
      {buildMsg && (
        <StatusLine className={buildClass}>
          <BuildStatusIcon>{buildIcon}</BuildStatusIcon>
          <BuildStatusMsg>{buildMsg}</BuildStatusMsg>
        </StatusLine>
      )}
      {runtimeMsg && (
        <StatusLine className={runtimeClass}>
          <RuntimeStatusIcon>{runtimeIcon}</RuntimeStatusIcon>
          <RuntimeStatusMsg>{runtimeMsg}</RuntimeStatusMsg>
        </StatusLine>
      )}
    </StyledStatus>
  )
}

export default function OverviewTable(props: OverviewTableProps) {
  const columns = columnDefs()
  const data = React.useMemo(
    () => props.view.uiResources?.map(uiResourceToCell) || [],
    [props.view.uiResources]
  )

  const {
    getTableProps,
    getTableBodyProps,
    headerGroups,
    rows,
    prepareRow,
  } = useTable(
    {
      columns,
      data,
      autoResetSortBy: false,
    },
    useSortBy
  )

  return (
    <ResourceTable {...getTableProps()}>
      <ResourceTableHead>
        {headerGroups.map((headerGroup) => (
          <ResourceTableRow {...headerGroup.getHeaderGroupProps()}>
            {headerGroup.headers.map((column) => (
              <ResourceTableHeader
                {...column.getHeaderProps([{style: {width: column.width }}, column.getSortByToggleProps()])}
              >
                {column.render("Header")}
                <ResourceTableHeaderSort
                  className={
                    column.isSorted
                      ? column.isSortedDesc
                        ? "is-sorted-desc"
                        : "is-sorted-asc"
                      : ""
                  }
                >
                  <ResourceTableHeaderSortTriangle />
                </ResourceTableHeaderSort>
              </ResourceTableHeader>
            ))}
          </ResourceTableRow>
        ))}
      </ResourceTableHead>
      <tbody {...getTableBodyProps()}>
        {rows.map((row, i) => {
          prepareRow(row)
          return (
            <ResourceTableRow {...row.getRowProps()}>
              {row.cells.map((cell) => (
                <ResourceTableData {...cell.getCellProps()}>
                  {cell.render("Cell")}
                </ResourceTableData>
              ))}
            </ResourceTableRow>
          )
        })}
      </tbody>
    </ResourceTable>
  )
}