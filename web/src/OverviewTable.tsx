import React from "react"
import { CellProps, Column, useTable } from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { useStarredResources } from "./StarredResourcesContext"
import { combinedStatus } from "./status"
import { Color, Font, FontSize } from "./style-helpers"
import { timeAgoFormatter } from "./timeFormatters"
import { ResourceStatus } from "./types"

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
}

function columnDefs(): Column<RowValues>[] {
  return React.useMemo(
    () => [
      {
        Header: "Star",
        accessor: "isStarred",
      },

      {
        Header: "Name",
        accessor: "name",
      },
      {
        Header: "Status",
        accessor: "status",
      },
      {
        Header: "Type",
        accessor: "resourceType",
      },
      {
        Header: "Ago",
        accessor: "lastUpdateTime",
        Cell: ({ row }: CellProps<RowValues>) => {
            row.values.lastUpdateTime ?? (
              <TimeAgo
                date={row.values.lastUpdateTime}
                formatter={timeAgoFormatter}
              />
            )
          )
        },
      },
    ],
    []
  )
}

function resourceViewToCell(r: webviewResource): RowValues {
  const starCtx = useStarredResources()
  return {
    isStarred: (r.name && starCtx.starredResources.includes(r.name)) || false,
    lastUpdateTime: r.lastDeployTime,
    status: combinedStatus(r),
    name: r.name || "",
    resourceType: "unknown",
  }
}

const ResourceTableRow = styled.tr`
  border-bottom: 1px solid ${Color.grayLighter};
`
const ResourceTableData = styled.td`
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  color: ${Color.gray6};
`
const ResourceTableHeader = styled(ResourceTableData)`
  color: ${Color.gray7};
`
const ResourceTableHead = styled.thead`
  background-color: ${Color.grayDarker};
`
const ResourceTable = styled.table`
  border-collapse: collapse;
`

export default function OverviewTable(props: Props) {
  const columns = columnDefs()
  const data = props.view.resources?.map(resourceViewToCell) || []

  const {
    getTableProps,
    getTableBodyProps,
    headerGroups,
    rows,
    prepareRow,
    visibleColumns,
  } = useTable({
    columns: columns,
    data,
  })

  return (
    <ResourceTable {...getTableProps()}>
      <ResourceTableHead>
        {headerGroups.map((headerGroup) => (
          <ResourceTableRow {...headerGroup.getHeaderGroupProps()}>
            {headerGroup.headers.map((column) => (
              <ResourceTableHeader {...column.getHeaderProps()}>
                {column.render("Header")}
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
