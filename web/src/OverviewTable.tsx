import React from "react"
import { CellProps, Column, useSortBy, useTable } from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { incr } from "./analytics"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { displayURL } from "./links"
import SidebarTriggerButton from "./SidebarTriggerButton"
import { useStarredResources } from "./StarredResourcesContext"
import StarResourceButton from "./StarResourceButton"
import { combinedStatus } from "./status"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { isZeroTime } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import { TriggerModeToggle } from "./TriggerModeToggle"
import { ResourceStatus, TargetType, TriggerMode } from "./types"

type webviewResource = Proto.webviewResource
type Props = {
  view: Proto.webviewView
}

type RowValues = {
  isStarred: boolean
  lastUpdateTime?: string
  status: ResourceStatus
  name: string
  resourceType: string
  triggerMode?: number
  podId?: string
  endpoints: Proto.webviewLink[]
}

let ResourceTable = styled.table`
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
`

let Endpoint = styled.a``

let DetailText = styled.div`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-left: 10px;
`

function columnDefs(): Column<RowValues>[] {
  return React.useMemo(
    () => [
      {
        Header: "Star",
        accessor: "isStarred",
        Cell: ({ row }: CellProps<RowValues>) => {
          return (
            <StarResourceButton
              resourceName={row.values.name}
              analyticsName="ui.web.overviewStarButton"
            />
          )
        },
      },
      {
        Header: "Last Updated",
        accessor: "lastUpdateTime",
        Cell: ({ row }: CellProps<RowValues>) => {
          return (
            <TimeAgo
              date={row.values.lastUpdateTime}
              formatter={timeAgoFormatter}
            />
          )
        },
      },
      {
        Header: "Trigger Button",
        Cell: ({ row }: CellProps<RowValues>) => {
          let building = !isZeroTime(row.values.currentBuildStartTime)
          let hasBuilt = row.values.lastBuild !== null
          let onTrigger = triggerUpdate.bind(null, row.values.name)

          return (
            <SidebarTriggerButton
              isTiltfile={row.values.isTiltfile}
              isSelected={false}
              hasPendingChanges={row.values.hasPendingChanges}
              hasBuilt={hasBuilt}
              isBuilding={building}
              triggerMode={row.values.triggerMode}
              isQueued={row.values.queued}
              onTrigger={onTrigger}
            />
          )
        },
      },
      {
        Header: "Name",
        accessor: "name",
      },
      {
        Header: "Type",
        accessor: "resourceType",
      },
      {
        Header: "Status",
        accessor: "status",
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
    []
  )
}

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

function resourceViewToCell(r: webviewResource): RowValues {
  const starCtx = useStarredResources()
  return {
    isStarred: (r.name && starCtx.starredResources.includes(r.name)) || false,
    lastUpdateTime: r.lastDeployTime,
    status: combinedStatus(r),
    name: r.name || "",
    triggerMode: r.triggerMode,
    resourceType: resourceTypeLabel(r),
    podId: r.podID,
    endpoints: r.endpointLinks ?? [],
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

function resourceTypeLabel(res: webviewResource): string {
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
      if (res.localResourceInfo && !!res.localResourceInfo.isTest) {
        return "Test"
      }
      return "Local Script"
    }
  }
  return "Unknown"
}

export default function OverviewTable(props: Props) {
  const columns = columnDefs()

  const data = React.useMemo(
    () => props.view.resources?.map(resourceViewToCell) || [],
    []
  )

  const {
    getTableProps,
    getTableBodyProps,
    headerGroups,
    rows,
    prepareRow,
  } = useTable(
    {
      columns: columns,
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
