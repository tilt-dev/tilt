import React from "react"
import { Link } from "react-router-dom"
import { CellProps, Column, useSortBy, useTable } from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { buildAlerts } from "./alerts"
import { incr } from "./analytics"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { displayURL } from "./links"
import { usePathBuilder } from "./PathBuilder"
import { useStarredResources } from "./StarredResourcesContext"
import { buildStatus } from "./status"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"
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
  buildStatusText: BuildStatusTextType
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

type BuildStatusTextType = {
  buildStatus: ResourceStatus
  buildAlertCount: number
  lastBuildDur: moment.Duration | null
}

let ResourceTable = styled.table`
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(1)};
  margin-right: ${SizeUnit(1)};
  border-collapse: collapse;
`
let ResourceTableHead = styled.thead`
  background-color: ${Color.grayDarker};
`
let ResourceTableRow = styled.tr`
  border-bottom: 1px solid ${Color.grayLighter};
`
let ResourceTableData = styled.td`
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  color: ${Color.gray6};
`
let ResourceTableHeader = styled(ResourceTableData)`
  color: ${Color.gray7};
  font-size: ${FontSize.smallest};
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
`

const ResourceNameLink = styled(Link)``

let Endpoint = styled.a``

let DetailText = styled.div`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-left: 10px;
`

function columnDefs(): Column<RowValues>[] {
  let ctx = useStarredResources()

  return React.useMemo(
    () => [
      {
        Header: "Starred",
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
        maxWidth: 1,
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
        maxWidth: 1,
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
        accessor: "name",
        Cell: ({ row }: CellProps<RowValues>) => {
          let pathBuilder = usePathBuilder()
          let link = pathBuilder.encpath`/r/${row.values.name}/overview`

          return (
            <ResourceNameLink to={link}>{row.values.name}</ResourceNameLink>
          )
        },
      },
      {
        Header: "Type",
        accessor: "resourceTypeLabel",
      },
      {
        Header: "Status",
        accessor: "buildStatusText",
        Cell: ({ row }: CellProps<RowValues>) => {
          return buildStatusText(
            row.values.buildStatusText.buildStatus,
            row.values.buildStatusText.lastBuildDur,
            row.values.buildStatusText.buildAlertCount
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
                <LinkSvg />
                <DetailText>{ep.name || displayURL(ep)}</DetailText>
              </Endpoint>
            )
          })
          return <div>{endpoints}</div>
        },
      },
      {
        Header: "Trigger Mode",
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
    buildStatusText: {
      buildStatus: buildStatus(r),
      lastBuildDur:
        lastBuild && lastBuild.startTime && lastBuild.finishTime
          ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
          : null,
      buildAlertCount: buildAlerts(r, null).length,
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

function buildStatusText(
  buildStatus: ResourceStatus,
  lastBuildDur: moment.Duration | null,
  buildAlertCount: number
) {
  console.log("lastbuilddur", lastBuildDur)
  let msg
  let icon
  let buildDurString = lastBuildDur
    ? ` in ${formatBuildDuration(lastBuildDur)}`
    : ""

  if (buildStatus === ResourceStatus.Pending) {
    icon = <LinkSvg />
    msg = "Pending"
  } else if (buildStatus === ResourceStatus.Building) {
    icon = <LinkSvg />
    msg = "Updating…"
  } else if (buildStatus === ResourceStatus.None) {
    icon = <LinkSvg />
    msg = "No update status"
  } else if (buildStatus === ResourceStatus.Unhealthy) {
    icon = <LinkSvg />
    msg = "Update error"
  } else if (buildStatus === ResourceStatus.Healthy) {
    icon = <LinkSvg />
    msg = `Completed${buildDurString}`
    if (buildAlertCount > 0) {
      msg += ", with issues"
    }
  } else {
    msg = "Unknown"
  }

  return (
    <>
      <span>{icon}</span>
      <span>{msg}</span>
    </>
  )
}

function runtimeStatusText(status: ResourceStatus): string {
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
      return ""
  }
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
                {...column.getHeaderProps(column.getSortByToggleProps())}
              >
                {column.render("Header")}
                <span>
                  {column.isSorted ? (column.isSortedDesc ? " 🔽" : " 🔼") : ""}
                </span>
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
