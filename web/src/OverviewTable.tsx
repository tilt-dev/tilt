import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import React, {
  ChangeEvent,
  MouseEvent,
  MutableRefObject,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react"
import {
  HeaderGroup,
  Row,
  SortingRule,
  TableHeaderProps,
  TableOptions,
  TableState,
  usePagination,
  useSortBy,
  UseSortByState,
  useTable,
} from "react-table"
import styled from "styled-components"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { AnalyticsType, Tags } from "./analytics"
import { ApiButtonType, buttonsForComponent } from "./ApiButton"
import {
  DEFAULT_RESOURCE_LIST_LIMIT,
  RESOURCE_LIST_MULTIPLIER,
} from "./constants"
import Features, { Flag, useFeatures } from "./feature"
import { Hold } from "./Hold"
import {
  getResourceLabels,
  GroupByLabelView,
  orderLabels,
  TILTFILE_LABEL,
  UNLABELED_LABEL,
} from "./labels"
import { LogAlertIndex, useLogAlertIndex } from "./LogStore"
import {
  getTableColumns,
  ResourceTableHeaderTip,
  rowIsDisabled,
  RowValues,
} from "./OverviewTableColumns"
import { OverviewTableDisplayOptions } from "./OverviewTableDisplayOptions"
import { OverviewTableKeyboardShortcuts } from "./OverviewTableKeyboardShortcuts"
import {
  AccordionDetailsStyleResetMixin,
  AccordionStyleResetMixin,
  AccordionSummaryStyleResetMixin,
  ResourceGroupsInfoTip,
  ResourceGroupSummaryIcon,
  ResourceGroupSummaryMixin,
} from "./ResourceGroups"
import { useResourceGroups } from "./ResourceGroupsContext"
import {
  ResourceListOptions,
  useResourceListOptions,
} from "./ResourceListOptionsContext"
import { matchesResourceName } from "./ResourceNameFilter"
import { useResourceSelection } from "./ResourceSelectionContext"
import { resourceIsDisabled, resourceTargetType } from "./ResourceStatus"
import { TableGroupStatusSummary } from "./ResourceStatusSummary"
import { ShowMoreButton } from "./ShowMoreButton"
import { buildStatus, runtimeStatus } from "./status"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { isZeroTime, timeDiff } from "./time"
import {
  ResourceName,
  ResourceStatus,
  TargetType,
  TriggerMode,
  UIButton,
  UIResource,
  UIResourceStatus,
} from "./types"

export type OverviewTableProps = {
  view: Proto.webviewView
}

type TableWrapperProps = {
  resources?: UIResource[]
  buttons?: UIButton[]
}

type TableGroupProps = {
  label: string
  setGlobalSortBy: (id: string) => void
  focused: string
} & TableOptions<RowValues>

type TableProps = {
  setGlobalSortBy?: (id: string) => void
  focused: string
} & TableOptions<RowValues>

type ResourceTableHeadRowProps = {
  headerGroup: HeaderGroup<RowValues>
  setGlobalSortBy?: (id: string) => void
} & TableHeaderProps

// Resource name filter styles
export const ResourceResultCount = styled.p`
  color: ${Color.gray50};
  font-size: ${FontSize.small};
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(0.5)};
  text-transform: uppercase;
`

export const NoMatchesFound = styled.p`
  color: ${Color.grayLightest};
  margin-left: ${SizeUnit(0.5)};
  margin-top: ${SizeUnit(1 / 4)};
`

// Table styles
const OverviewTableRoot = styled.section`
  padding-bottom: ${SizeUnit(1 / 2)};
  margin-left: auto;
  margin-right: auto;
  /* Max and min width are based on fixed table layout and column widths */
  max-width: 2000px;
  min-width: 1400px;

  @media screen and (max-width: 2200px) {
    margin-left: ${SizeUnit(1 / 2)};
    margin-right: ${SizeUnit(1 / 2)};
  }
`

const TableWithoutGroupsRoot = styled.div`
  box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.25);
  border: 1px ${Color.gray40} solid;
  border-radius: 0px 0px 8px 8px;
  background-color: ${Color.gray20};
`

const ResourceTable = styled.table`
  table-layout: fixed;
  width: 100%;
  border-spacing: 0;
  border-collapse: collapse;

  td {
    padding-left: 10px;
    padding-right: 10px;
  }

  td:first-child {
    padding-left: ${SizeUnit(0.75)};
  }

  td:last-child {
    padding-right: ${SizeUnit(1)};
  }
`
const ResourceTableHead = styled.thead`
  & > tr {
    background-color: ${Color.gray10};
  }
`

export const ResourceTableRow = styled.tr`
  border-top: 1px solid ${Color.gray40};
  border-left: 4px solid transparent;
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  font-style: none;
  color: ${Color.gray60};
  padding-top: 6px;
  padding-bottom: 6px;

  &.isFocused,
  &:focus {
    border-left: 4px solid ${Color.blue};
    outline: none;
  }

  &.isSelected {
    background-color: ${Color.gray30};
  }

  /* For visual consistency on rows */
  &.isFixedHeight {
    height: ${SizeUnit(1.4)};
  }
`
export const ResourceTableData = styled.td`
  box-sizing: border-box;

  &.isSorted {
    background-color: ${Color.gray30};
  }

  &.alignRight {
    text-align: right;
  }
`

export const ResourceTableHeader = styled(ResourceTableData)`
  color: ${Color.gray70};
  font-size: ${FontSize.small};
  box-sizing: border-box;
  white-space: nowrap;

  &.isSorted {
    background-color: ${Color.gray20};
  }
`

const ResourceTableHeaderLabel = styled.div`
  display: flex;
  align-items: center;
  user-select: none;
`

export const ResourceTableHeaderSortTriangle = styled.div`
  display: inline-block;
  margin-left: ${SizeUnit(0.25)};
  width: 0;
  height: 0;
  border-left: 5px solid transparent;
  border-right: 5px solid transparent;
  border-bottom: 6px solid ${Color.gray50};

  &.is-sorted-asc {
    border-bottom: 6px solid ${Color.blue};
  }
  &.is-sorted-desc {
    border-bottom: 6px solid ${Color.blue};
    transform: rotate(180deg);
  }
`

// Table Group styles
export const OverviewGroup = styled(Accordion)`
  ${AccordionStyleResetMixin}
  color: ${Color.gray50};
  border: 1px ${Color.gray40} solid;
  background-color: ${Color.gray20};

  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded {
    margin-top: ${SizeUnit(1 / 2)};
  }

  &.Mui-expanded {
    box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.25);
    border-radius: 0px 0px 8px 8px;
  }
`

export const OverviewGroupSummary = styled(AccordionSummary)`
  ${AccordionSummaryStyleResetMixin}
  ${ResourceGroupSummaryMixin}
  background-color: ${Color.gray10};

  .MuiAccordionSummary-content {
    font-size: ${FontSize.default};
  }
`

export const OverviewGroupName = styled.span`
  padding: 0 ${SizeUnit(1 / 3)};
`

export const OverviewGroupDetails = styled(AccordionDetails)`
  ${AccordionDetailsStyleResetMixin}
`
const TABLE_TYPE_TAGS: Tags = { type: AnalyticsType.Grid }

const GROUP_INFO_TOOLTIP_ID = "table-groups-info"

export function TableResourceResultCount(props: { resources?: UIResource[] }) {
  const { options } = useResourceListOptions()

  if (
    props.resources === undefined ||
    options.resourceNameFilter.length === 0
  ) {
    return null
  }

  const count = props.resources.length

  return (
    <ResourceResultCount>
      {count} result{count !== 1 ? "s" : ""}
    </ResourceResultCount>
  )
}

export function TableNoMatchesFound(props: { resources?: UIResource[] }) {
  const { options } = useResourceListOptions()

  if (props.resources?.length === 0 && options.resourceNameFilter.length > 0) {
    return <NoMatchesFound>No matching resources</NoMatchesFound>
  }

  return null
}

const FIRST_SORT_STATE = false
const SECOND_SORT_STATE = true

// This helper function manually implements the toggle sorting
// logic used by react-table, so we can keep the sorting state
// globally and sort multiple tables by the same column.
//    Click once to sort by ascending values
//    Click twice to sort by descending values
//    Click thrice to remove sort
// Note: this does NOT support sorting by multiple columns.
function calculateNextSort(
  id: string,
  sortByState: SortingRule<RowValues>[] | undefined
): SortingRule<RowValues>[] {
  if (!sortByState || sortByState.length === 0) {
    return [{ id, desc: FIRST_SORT_STATE }]
  }

  // If the current sort is the same column as next sort,
  // determine its next value
  const [currentSort] = sortByState
  if (currentSort.id === id) {
    const { desc } = currentSort

    if (desc === undefined) {
      return [{ id, desc: FIRST_SORT_STATE }]
    }

    if (desc === FIRST_SORT_STATE) {
      return [{ id, desc: SECOND_SORT_STATE }]
    }

    if (desc === SECOND_SORT_STATE) {
      return []
    }
  }

  return [{ id, desc: FIRST_SORT_STATE }]
}

function applyOptionsToResources(
  resources: UIResource[] | undefined,
  options: ResourceListOptions,
  features: Features
): UIResource[] {
  if (!resources) {
    return []
  }

  const hideDisabledResources =
    !features.isEnabled(Flag.DisableResources) || !options.showDisabledResources
  const resourceNameFilter = options.resourceNameFilter.length > 0

  // If there are no options to apply to the resources, return the un-filtered, sorted list
  if (!resourceNameFilter && !hideDisabledResources) {
    return sortByDisableStatus(resources)
  }

  // Otherwise, apply the options to the resources and sort it
  const filteredResources = resources.filter((r) => {
    const resourceDisabled = resourceIsDisabled(r)
    if (hideDisabledResources && resourceDisabled) {
      return false
    }

    if (resourceNameFilter) {
      return matchesResourceName(
        r.metadata?.name || "",
        options.resourceNameFilter
      )
    }

    return true
  })

  return sortByDisableStatus(filteredResources)
}

function uiResourceToCell(
  r: UIResource,
  allButtons: UIButton[] | undefined,
  alertIndex: LogAlertIndex
): RowValues {
  let res = (r.status || {}) as UIResourceStatus
  let buildHistory = res.buildHistory || []
  let lastBuild = buildHistory.length > 0 ? buildHistory[0] : null
  let lastBuildDur =
    lastBuild?.startTime && lastBuild?.finishTime
      ? timeDiff(lastBuild.startTime, lastBuild.finishTime)
      : null
  let currentBuildStartTime = res.currentBuild?.startTime ?? ""
  let isBuilding = !isZeroTime(currentBuildStartTime)
  let hasBuilt = lastBuild !== null
  let buttons = buttonsForComponent(
    allButtons,
    ApiButtonType.Resource,
    r.metadata?.name
  )
  let analyticsTags = { target: resourceTargetType(r) }
  // Consider a resource `selectable` if it can be disabled
  const selectable = !!buttons.toggleDisable

  return {
    lastDeployTime: res.lastDeployTime ?? "",
    trigger: {
      isBuilding: isBuilding,
      hasBuilt: hasBuilt,
      hasPendingChanges: !!res.hasPendingChanges,
      isQueued: !!res.queued,
    },
    name: r.metadata?.name ?? "",
    resourceTypeLabel: resourceTypeLabel(r),
    statusLine: {
      buildStatus: buildStatus(r, alertIndex),
      buildAlertCount: buildAlerts(r, alertIndex).length,
      lastBuildDur: lastBuildDur,
      runtimeStatus: runtimeStatus(r, alertIndex),
      runtimeAlertCount: runtimeAlerts(r, alertIndex).length,
      hold: res.waiting ? new Hold(res.waiting) : null,
    },
    podId: res.k8sResourceInfo?.podName ?? "",
    endpoints: res.endpointLinks ?? [],
    triggerMode: res.triggerMode ?? TriggerMode.TriggerModeAuto,
    buttons: buttons,
    analyticsTags: analyticsTags,
    selectable,
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
      return "K8s"
    } else if (spec.type === TargetType.DockerCompose) {
      return "DCS"
    } else if (spec.type === TargetType.Local) {
      return "Local"
    }
  }
  return "Unknown"
}

function sortByDisableStatus(resources: UIResource[] = []) {
  // Sort by disabled status, so disabled resources appear at the end of each table list.
  // Note: this initial sort is done here so it doesn't interfere with the sorting
  // managed by react-table
  const sorted = [...resources].sort((a, b) => {
    const resourceAOrder = resourceIsDisabled(a) ? 1 : 0
    const resourceBOrder = resourceIsDisabled(b) ? 1 : 0

    return resourceAOrder - resourceBOrder
  })

  return sorted
}

function onlyEnabledRows(rows: RowValues[]): RowValues[] {
  return rows.filter(
    (row) => row.statusLine.runtimeStatus !== ResourceStatus.Disabled
  )
}
function onlyDisabledRows(rows: RowValues[]): RowValues[] {
  return rows.filter(
    (row) => row.statusLine.runtimeStatus === ResourceStatus.Disabled
  )
}
function enabledRowsFirst(rows: RowValues[]): RowValues[] {
  let result = onlyEnabledRows(rows)
  result.push(...onlyDisabledRows(rows))
  return result
}

export function labeledResourcesToTableCells(
  resources: UIResource[] | undefined,
  buttons: UIButton[] | undefined,
  logAlertIndex: LogAlertIndex
): GroupByLabelView<RowValues> {
  const labelsToResources: { [key: string]: RowValues[] } = {}
  const unlabeled: RowValues[] = []
  const tiltfile: RowValues[] = []

  if (resources === undefined) {
    return { labels: [], labelsToResources, tiltfile, unlabeled }
  }

  resources.forEach((r) => {
    const labels = getResourceLabels(r)
    const isTiltfile = r.metadata?.name === ResourceName.tiltfile
    const tableCell = uiResourceToCell(r, buttons, logAlertIndex)
    if (labels.length) {
      labels.forEach((label) => {
        if (!labelsToResources.hasOwnProperty(label)) {
          labelsToResources[label] = []
        }

        labelsToResources[label].push(tableCell)
      })
    } else if (isTiltfile) {
      tiltfile.push(tableCell)
    } else {
      unlabeled.push(tableCell)
    }
  })

  // Labels are always displayed in sorted order
  const labels = orderLabels(Object.keys(labelsToResources))

  return { labels, labelsToResources, tiltfile, unlabeled }
}

export function ResourceTableHeadRow({
  headerGroup,
  setGlobalSortBy,
}: ResourceTableHeadRowProps) {
  const calculateToggleProps = (column: HeaderGroup<RowValues>) => {
    // If a column header is JSX, fall back on using its id as a descriptive title
    // and capitalize for consistency
    const columnHeader =
      typeof column.Header === "string"
        ? column.Header
        : `${column.id[0]?.toUpperCase()}${column.id?.slice(1)}`

    // Warning! Toggle props are not typed or documented well within react-table.
    // Modify toggle props with caution.
    // See https://react-table.tanstack.com/docs/api/useSortBy#column-properties
    const toggleProps: { [key: string]: any } = {
      title: column.canSort ? `Sort by ${columnHeader}` : columnHeader,
    }

    if (setGlobalSortBy && column.canSort) {
      // The sort state is global whenever there are multiple tables, so
      // pass a click handler to the sort toggle that changes the global state
      toggleProps.onClick = () => setGlobalSortBy(column.id)
    }

    return toggleProps
  }

  const calculateHeaderProps = (column: HeaderGroup<RowValues>) => {
    const headerProps: Partial<TableHeaderProps> = {
      style: { width: column.width },
    }

    if (column.isSorted) {
      headerProps.className = "isSorted"
    }

    return headerProps
  }

  return (
    <ResourceTableRow>
      {headerGroup.headers.map((column) => (
        <ResourceTableHeader
          {...column.getHeaderProps([
            calculateHeaderProps(column),
            column.getSortByToggleProps(calculateToggleProps(column)),
          ])}
        >
          <ResourceTableHeaderLabel>
            {column.render("Header")}
            <ResourceTableHeaderTip id={String(column.id)} />
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
          </ResourceTableHeaderLabel>
        </ResourceTableHeader>
      ))}
    </ResourceTableRow>
  )
}

function ShowMoreResourcesRow({
  colSpan,
  itemCount,
  pageSize,
  onClick,
}: {
  colSpan: number
  itemCount: number
  pageSize: number
  onClick: (e: MouseEvent) => void
}) {
  if (itemCount <= pageSize) {
    return null
  }

  return (
    <ResourceTableRow className="isFixedHeight">
      <ResourceTableData colSpan={colSpan - 2} />
      <ResourceTableData className="alignRight" colSpan={2}>
        <ShowMoreButton
          itemCount={itemCount}
          currentListSize={pageSize}
          onClick={onClick}
          analyticsTags={TABLE_TYPE_TAGS}
        />
      </ResourceTableData>
    </ResourceTableRow>
  )
}

function TableRow(props: { row: Row<RowValues>; focused: string }) {
  let { row, focused } = props
  const { isSelected } = useResourceSelection()
  let isFocused = row.original.name == focused
  let rowClasses =
    (rowIsDisabled(row) ? "isDisabled " : "") +
    (isSelected(row.original.name) ? "isSelected " : "") +
    (isFocused ? "isFocused " : "")
  let ref: MutableRefObject<HTMLTableRowElement | null> = useRef(null)

  useEffect(() => {
    if (isFocused && ref.current) {
      ref.current.focus()
    }
  }, [isFocused, ref])

  return (
    <ResourceTableRow
      tabIndex={-1}
      ref={ref}
      {...row.getRowProps({
        className: rowClasses,
      })}
    >
      {row.cells.map((cell) => (
        <ResourceTableData
          {...cell.getCellProps()}
          className={cell.column.isSorted ? "isSorted" : ""}
        >
          {cell.render("Cell")}
        </ResourceTableData>
      ))}
    </ResourceTableRow>
  )
}

export function Table(props: TableProps) {
  if (props.data.length === 0) {
    return null
  }

  const {
    getTableProps,
    getTableBodyProps,
    headerGroups,
    rows, // Used to calculate the total number of rows
    page, // Used to render the rows for the current page
    prepareRow,
    state: { pageSize },
    setPageSize,
  } = useTable(
    {
      columns: props.columns,
      data: props.data,
      autoResetSortBy: false,
      useControlledState: props.useControlledState,
      initialState: { pageSize: DEFAULT_RESOURCE_LIST_LIMIT },
    },
    useSortBy,
    usePagination
  )

  const showMoreOnClick = () => setPageSize(pageSize * RESOURCE_LIST_MULTIPLIER)

  // TODO (lizz): Consider adding `aria-sort` markup to table headings
  return (
    <ResourceTable {...getTableProps()}>
      <ResourceTableHead>
        {headerGroups.map((headerGroup: HeaderGroup<RowValues>) => (
          <ResourceTableHeadRow
            {...headerGroup.getHeaderGroupProps()}
            headerGroup={headerGroup}
            setGlobalSortBy={props.setGlobalSortBy}
          />
        ))}
      </ResourceTableHead>
      <tbody {...getTableBodyProps()}>
        {page.map((row: Row<RowValues>) => {
          prepareRow(row)
          return (
            <TableRow
              key={row.original.name}
              row={row}
              focused={props.focused}
            />
          )
        })}
        <ShowMoreResourcesRow
          itemCount={rows.length}
          pageSize={pageSize}
          onClick={showMoreOnClick}
          colSpan={props.columns.length}
        />
      </tbody>
    </ResourceTable>
  )
}

function TableGroup(props: TableGroupProps) {
  const { label, ...tableProps } = props

  if (tableProps.data.length === 0) {
    return null
  }

  const formattedLabel = label === UNLABELED_LABEL ? <em>{label}</em> : label
  const labelNameId = `tableOverview-${label}`

  const { getGroup, toggleGroupExpanded } = useResourceGroups()
  const { expanded } = getGroup(label)
  const handleChange = (_e: ChangeEvent<{}>) =>
    toggleGroupExpanded(label, AnalyticsType.Grid)

  return (
    <OverviewGroup expanded={expanded} onChange={handleChange}>
      <OverviewGroupSummary id={labelNameId}>
        <ResourceGroupSummaryIcon role="presentation" />
        <OverviewGroupName>{formattedLabel}</OverviewGroupName>
        <TableGroupStatusSummary
          labelText={`Status summary for ${label} group`}
          resources={tableProps.data}
        />
      </OverviewGroupSummary>
      <OverviewGroupDetails>
        <Table {...tableProps} />
      </OverviewGroupDetails>
    </OverviewGroup>
  )
}

export function TableGroupedByLabels({
  resources,
  buttons,
}: TableWrapperProps) {
  const features = useFeatures()
  const logAlertIndex = useLogAlertIndex()
  const data = useMemo(
    () => labeledResourcesToTableCells(resources, buttons, logAlertIndex),
    [resources, buttons]
  )

  const totalOrder = []
  data.labels.forEach((label) =>
    totalOrder.push(...enabledRowsFirst(data.labelsToResources[label]))
  )
  totalOrder.push(...enabledRowsFirst(data.tiltfile))
  totalOrder.push(...enabledRowsFirst(data.unlabeled))
  let firstResourceName = totalOrder[0]?.name
  let [focused, setFocused] = useState(firstResourceName || "")

  const columns = getTableColumns(features)

  // Global table settings are currently used to sort multiple
  // tables by the same column
  // See: https://react-table.tanstack.com/docs/faq#how-can-i-manually-control-the-table-state
  const [globalTableSettings, setGlobalTableSettings] =
    useState<UseSortByState<RowValues>>()

  const useControlledState = (state: TableState<RowValues>) =>
    useMemo(() => {
      return { ...state, ...globalTableSettings }
    }, [state, globalTableSettings])

  const setGlobalSortBy = (columnId: string) => {
    const sortBy = calculateNextSort(columnId, globalTableSettings?.sortBy)
    setGlobalTableSettings({ sortBy })
  }

  return (
    <>
      {data.labels.map((label) => (
        <TableGroup
          key={label}
          label={label}
          data={data.labelsToResources[label]}
          columns={columns}
          useControlledState={useControlledState}
          setGlobalSortBy={setGlobalSortBy}
          focused={focused}
        />
      ))}
      <TableGroup
        label={UNLABELED_LABEL}
        data={data.unlabeled}
        columns={columns}
        useControlledState={useControlledState}
        setGlobalSortBy={setGlobalSortBy}
        focused={focused}
      />
      <TableGroup
        label={TILTFILE_LABEL}
        data={data.tiltfile}
        columns={columns}
        useControlledState={useControlledState}
        setGlobalSortBy={setGlobalSortBy}
        focused={focused}
      />
      <OverviewTableKeyboardShortcuts
        focused={focused}
        setFocused={setFocused}
        rows={totalOrder}
      />
    </>
  )
}

export function TableWithoutGroups({ resources, buttons }: TableWrapperProps) {
  const features = useFeatures()
  const logAlertIndex = useLogAlertIndex()
  const data = useMemo(() => {
    return (
      resources?.map((r) => uiResourceToCell(r, buttons, logAlertIndex)) || []
    )
  }, [resources, buttons])
  const columns = getTableColumns(features)

  let totalOrder = enabledRowsFirst(data)
  let firstResourceName = totalOrder[0]?.name
  let [focused, setFocused] = useState(firstResourceName || "")

  if (resources?.length === 0) {
    return null
  }

  return (
    <TableWithoutGroupsRoot>
      <Table columns={columns} data={data} focused={focused} />
      <OverviewTableKeyboardShortcuts
        focused={focused}
        setFocused={setFocused}
        rows={totalOrder}
      />
    </TableWithoutGroupsRoot>
  )
}

function OverviewTableContent(props: OverviewTableProps) {
  const features = useFeatures()
  const labelsEnabled = features.isEnabled(Flag.Labels)
  const resourcesHaveLabels =
    props.view.uiResources?.some((r) => getResourceLabels(r).length > 0) ||
    false

  const { options } = useResourceListOptions()
  const resourceFilterApplied = options.resourceNameFilter.length > 0

  // Apply any display filters or options to resources, plus sort for initial view
  const resourcesToDisplay = applyOptionsToResources(
    props.view.uiResources,
    options,
    features
  )

  // Table groups are displayed when feature is enabled, resources have labels,
  // and no resource name filter is applied
  const displayResourceGroups =
    labelsEnabled && resourcesHaveLabels && !resourceFilterApplied

  if (displayResourceGroups) {
    return (
      <TableGroupedByLabels
        resources={resourcesToDisplay}
        buttons={props.view.uiButtons}
      />
    )
  } else {
    // The label group tip is only displayed if labels are enabled but not used
    const displayLabelGroupsTip = labelsEnabled && !resourcesHaveLabels

    return (
      <>
        {displayLabelGroupsTip && (
          <ResourceGroupsInfoTip idForIcon={GROUP_INFO_TOOLTIP_ID} />
        )}
        <TableResourceResultCount resources={resourcesToDisplay} />
        <TableNoMatchesFound resources={resourcesToDisplay} />
        <TableWithoutGroups
          aria-describedby={
            displayLabelGroupsTip ? GROUP_INFO_TOOLTIP_ID : undefined
          }
          resources={resourcesToDisplay}
          buttons={props.view.uiButtons}
        />
      </>
    )
  }
}

export default function OverviewTable(props: OverviewTableProps) {
  return (
    <OverviewTableRoot aria-label="Resources overview">
      <OverviewTableContent {...props} />
    </OverviewTableRoot>
  )
}
