import React from "react"
import { CellProps, Column, useSortBy, useTable } from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { incr } from "./analytics"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { displayURL } from "./links"
import OverviewTableStatus from "./OverviewTableStatus"
import { useResourceNav } from "./ResourceNav"
import { useStarredResources } from "./StarredResourcesContext"
import { buildStatus, runtimeStatus } from "./status"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"
import TableStarResourceButton from "./TableStarResourceButton"
import TableTriggerModeToggle from "./TableTriggerModeToggle"
import { isZeroTime, timeDiff } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import TriggerButton from "./TriggerButton"
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
  statusLine: StatusLineType
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

type StatusLineType = {
  buildStatus: ResourceStatus
  buildAlertCount: number
  lastBuildDur: moment.Duration | null
  runtimeStatus: ResourceStatus
  runtimeAlertCount: number
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
  box-sizing: border-box;
`
const ResourceTableHeader = styled(ResourceTableData)`
  color: ${Color.gray7};
  font-size: ${FontSize.smallester};
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  box-sizing: border-box;
`

const ResourceTableHeaderSortTriangle = styled.div`
  display: inline-block;
  margin-left: ${SizeUnit(0.25)};
  width: 0;
  height: 0;
  border-left: 4px solid transparent;
  border-right: 4px solid transparent;
  border-bottom: 6px solid ${Color.gray};

  &.is-sorted-asc {
    border-bottom: 6px solid ${Color.grayLightest};
  }
  &.is-sorted-desc {
    border-bottom: 6px solid ${Color.grayLightest};
    transform: rotate(180deg);
  }
`
const ResourceName = styled.div`
  color: ${Color.offWhite};
  font-size: ${FontSize.small};
  padding-top: ${SizeUnit(1 / 3)};
  padding-bottom: ${SizeUnit(1 / 3)};
  cursor: pointer;

  &:hover {
    text-decoration: underline;
    text-underline-position: under;
  }

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
        width: "20px",
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
        accessor: "hasPendingChanges",
        disableSortBy: true,
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
        width: "280px",
        accessor: "name",
        Cell: ({ row }: CellProps<RowValues>) => {
          let nav = useResourceNav()
          let hasError =
            row.values.statusLine.buildStatus === ResourceStatus.Unhealthy ||
            row.values.statusLine.runtimeStatus === ResourceStatus.Unhealthy

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
        width: "150px",
        accessor: "resourceTypeLabel",
      },
      {
        Header: "Status",
        width: "150px",
        accessor: "statusLine",
        Cell: ({ row }: CellProps<RowValues>) => {
          return (
            <>
              <OverviewTableStatus
                status={row.values.statusLine.buildStatus}
                lastBuildDur={row.values.statusLine.lastBuildDur}
                alertCount={row.values.statusLine.buildAlertCount}
                isBuild={true}
              />
              <OverviewTableStatus
                status={row.values.statusLine.runtimeStatus}
                alertCount={row.values.statusLine.runtimeAlertCount}
              />
            </>
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
        accessor: "triggerMode",
        width: "50px",
        Cell: ({ row }: CellProps<RowValues>) => {
          return (
            <TableTriggerModeToggle
              resourceName={row.values.name}
              triggerMode={row.values.triggerMode}
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
    statusLine: {
      buildStatus: buildStatus(r),
      lastBuildDur:
        lastBuild && lastBuild.startTime && lastBuild.finishTime
          ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
          : null,
      buildAlertCount: buildAlerts(r, null).length,
      runtimeAlertCount: runtimeAlerts(r, null).length,
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
                {...column.getHeaderProps([
                  { style: { width: column.width } },
                  column.getSortByToggleProps(),
                ])}
              >
                {column.render("Header")}
                {column.canSort && (
                  <ResourceTableHeaderSortTriangle
                    className={
                      column.isSorted
                        ? column.isSortedDesc
                          ? "is-sorted-desc"
                          : "is-sorted-asc"
                        : ""
                    }
                  />
                )}
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
