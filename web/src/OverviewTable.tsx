import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import React, { ChangeEvent, useMemo, useState } from "react"
import {
  CellProps,
  Column,
  HeaderGroup,
  Row,
  SortingRule,
  TableHeaderProps,
  TableOptions,
  TableState,
  useSortBy,
  useTable,
} from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { AnalyticsAction, AnalyticsType, incr } from "./analytics"
import { ApiButton, ApiIcon, buttonsForComponent } from "./ApiButton"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import { Flag, useFeatures } from "./feature"
import { InstrumentedButton } from "./instrumentedComponents"
import {
  getResourceLabels,
  GroupByLabelView,
  orderLabels,
  TILTFILE_LABEL,
  UNLABELED_LABEL,
} from "./labels"
import { displayURL } from "./links"
import { LogAlertIndex, useLogAlertIndex } from "./LogStore"
import { OverviewButtonMixin } from "./OverviewButton"
import OverviewTableStarResourceButton, {
  StyledTableStarResourceButton,
} from "./OverviewTableStarResourceButton"
import OverviewTableStatus from "./OverviewTableStatus"
import OverviewTableTriggerButton from "./OverviewTableTriggerButton"
import OverviewTableTriggerModeToggle from "./OverviewTableTriggerModeToggle"
import {
  AccordionDetailsStyleResetMixin,
  AccordionStyleResetMixin,
  AccordionSummaryStyleResetMixin,
  ResourceGroupsInfoTip,
  ResourceGroupSummaryIcon,
  ResourceGroupSummaryMixin,
} from "./ResourceGroups"
import { useResourceGroups } from "./ResourceGroupsContext"
import { useResourceNav } from "./ResourceNav"
import { TableGroupStatusSummary } from "./ResourceStatusSummary"
import { useStarredResources } from "./StarredResourcesContext"
import { buildStatus, runtimeStatus } from "./status"
import {
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import { isZeroTime, timeDiff } from "./time"
import { timeAgoFormatter } from "./timeFormatters"
import TiltTooltip, { TiltInfoTooltip } from "./Tooltip"
import {
  ResourceName,
  ResourceStatus,
  TargetType,
  TriggerMode,
  UIButton,
  UILink,
  UIResource,
  UIResourceStatus,
} from "./types"

export type OverviewTableProps = {
  view: Proto.webviewView
}

type TableGroupProps = {
  label: string
  setGlobalSortBy: (id: string) => void
} & TableOptions<RowValues>

type TableProps = {
  isGroupView?: boolean
  setGlobalSortBy?: (id: string) => void
} & TableOptions<RowValues>

type TableHeadRowProps = {
  headerGroup: HeaderGroup<RowValues>
  setGlobalSortBy?: (id: string) => void
} & TableHeaderProps

export type RowValues = {
  lastDeployTime: string
  trigger: OverviewTableTrigger
  name: string
  resourceTypeLabel: string
  statusLine: OverviewTableStatus
  podId: string
  endpoints: UILink[]
  triggerMode: TriggerMode
  buttons: UIButton[]
}

type OverviewTableTrigger = {
  isBuilding: boolean
  hasBuilt: boolean
  hasPendingChanges: boolean
  isQueued: boolean
}

type OverviewTableStatus = {
  buildStatus: ResourceStatus
  buildAlertCount: number
  lastBuildDur: moment.Duration | null
  runtimeStatus: ResourceStatus
  runtimeAlertCount: number
}

// Table styles
const OverviewTableRoot = styled.section`
  margin-bottom: ${SizeUnit(1 / 2)};
`

const ResourceTable = styled.table`
  margin-top: ${SizeUnit(0.5)};
  border-collapse: collapse;
  width: 100%;

  td:first-child {
    padding-left: ${SizeUnit(1)};

    & ${StyledTableStarResourceButton} {
      margin-left: -15px; /* Center the star button underneath the header */
    }
  }
  td:last-child {
    padding-right: ${SizeUnit(1)};
  }

  td + td {
    padding-left: ${SizeUnit(1 / 4)};
    padding-right: ${SizeUnit(1 / 4)};
  }

  &.isGroup {
    border: 1px ${Color.grayLighter} solid;
    border-radius: 0 ${SizeUnit(1 / 4)};
  }
`
const ResourceTableHead = styled.thead`
  background-color: ${Color.grayDarker};
`
export const ResourceTableRow = styled.tr`
  border-bottom: 1px solid ${Color.grayLighter};
`
export const ResourceTableData = styled.td`
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  color: ${Color.gray6};
  box-sizing: border-box;

  &.isSorted {
    background-color: ${Color.gray};
  }
`
export const ResourceTableHeader = styled(ResourceTableData)`
  color: ${Color.gray7};
  font-size: ${FontSize.small};
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  box-sizing: border-box;
  white-space: nowrap;

  &.isSorted {
    background-color: ${Color.grayDark};
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
  border-bottom: 6px solid ${Color.grayLight};

  &.is-sorted-asc {
    border-bottom: 6px solid ${Color.blue};
  }
  &.is-sorted-desc {
    border-bottom: 6px solid ${Color.blue};
    transform: rotate(180deg);
  }
`

const TableHeaderStarIcon = styled(StarSvg)`
  fill: ${Color.gray7};
  height: 13px;
  width: 13px;
`

const Name = styled.button`
  ${mixinResetButtonStyle};
  color: ${Color.offWhite};
  font-size: ${FontSize.small};
  padding-top: ${SizeUnit(1 / 3)};
  padding-bottom: ${SizeUnit(1 / 3)};
  text-align: left;
  cursor: pointer;

  &:hover {
    text-decoration: underline;
    text-underline-position: under;
  }

  &.has-error {
    color: ${Color.red};
  }
`

const Endpoint = styled.a`
  display: flex;
  align-items: center;
  max-width: 150px;
`
const DetailText = styled.div`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`

const StyledLinkSvg = styled(LinkSvg)`
  fill: ${Color.grayLight};
  margin-right: ${SizeUnit(0.2)};
`

const PodId = styled.div`
  display: flex;
  align-items: center;
`
const PodIdInput = styled.input`
  background-color: transparent;
  color: ${Color.gray6};
  font-family: inherit;
  font-size: inherit;
  border: 1px solid ${Color.grayDarkest};
  border-radius: 2px;
  padding: ${SizeUnit(0.1)} ${SizeUnit(0.2)};
  width: 100px;

  &::selection {
    background-color: ${Color.gray};
  }
`
const PodIdCopy = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  padding-top: ${SizeUnit(0.5)};
  padding: ${SizeUnit(0.25)};

  svg {
    fill: ${Color.gray6};
  }
`
const CustomActionButton = styled(ApiButton)`
  button {
    ${OverviewButtonMixin};
  }
`
const WidgetCell = styled.span`
  display: flex;
  flex-wrap: wrap;
  max-width: ${SizeUnit(8)};

  .MuiButtonGroup-root {
    margin-bottom: ${SizeUnit(0.125)};
    margin-right: ${SizeUnit(0.125)};
  }
`

// Table Group styles
export const OverviewGroup = styled(Accordion)`
  ${AccordionStyleResetMixin}

  /* Set specific margins for table view */
  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded,
  &.MuiAccordion-root.Mui-expanded:first-child {
    margin: ${SizeUnit(1 / 2)};
  }
`

export const OverviewGroupSummary = styled(AccordionSummary)`
  ${AccordionSummaryStyleResetMixin}
  ${ResourceGroupSummaryMixin}

  .MuiAccordionSummary-content {
    font-size: ${FontSize.default};
  }
`

export const OverviewGroupName = styled.span`
  padding: 0 ${SizeUnit(1 / 3)};
`

export const OverviewGroupDetails = styled(AccordionDetails)`
  ${AccordionDetailsStyleResetMixin}

  ${ResourceTable} {
    margin-top: 4px;
  }
`

const GROUP_INFO_TOOLTIP_ID = "table-groups-info"

function TableStarColumn({ row }: CellProps<RowValues>) {
  let ctx = useStarredResources()
  return (
    <OverviewTableStarResourceButton
      resourceName={row.values.name}
      analyticsName="ui.web.overviewStarButton"
      ctx={ctx}
    />
  )
}

function TableUpdateColumn({ row }: CellProps<RowValues>) {
  if (!row.values.lastDeployTime) {
    return null
  }
  return (
    <TimeAgo date={row.values.lastDeployTime} formatter={timeAgoFormatter} />
  )
}

function TableTriggerColumn({ row }: CellProps<RowValues>) {
  const trigger = row.original.trigger
  return (
    <OverviewTableTriggerButton
      hasPendingChanges={trigger.hasPendingChanges}
      hasBuilt={trigger.hasBuilt}
      isBuilding={trigger.isBuilding}
      triggerMode={row.values.triggerMode}
      isQueued={trigger.isQueued}
      resourceName={row.values.name}
    />
  )
}

export function TableNameColumn({ row }: CellProps<RowValues>) {
  let nav = useResourceNav()
  let hasError =
    row.original.statusLine.buildStatus === ResourceStatus.Unhealthy ||
    row.original.statusLine.runtimeStatus === ResourceStatus.Unhealthy

  return (
    <Name
      className={hasError ? "has-error" : ""}
      onClick={(e) => nav.openResource(row.values.name)}
    >
      {row.values.name}
    </Name>
  )
}

function TableStatusColumn({ row }: CellProps<RowValues>) {
  const status = row.original.statusLine
  return (
    <>
      <OverviewTableStatus
        status={status.buildStatus}
        lastBuildDur={status.lastBuildDur}
        isBuild={true}
        resourceName={row.values.name}
      />
      <OverviewTableStatus
        status={status.runtimeStatus}
        resourceName={row.values.name}
      />
    </>
  )
}

function TablePodIDColumn({ row }: CellProps<RowValues>) {
  let [showCopySuccess, setShowCopySuccess] = useState(false)

  let copyClick = () => {
    copyTextToClipboard(row.values.podId, () => {
      setShowCopySuccess(true)

      setTimeout(() => {
        setShowCopySuccess(false)
      }, 3000)
    })
  }

  let icon = showCopySuccess ? (
    <CheckmarkSvg width="15" height="15" />
  ) : (
    <CopySvg width="15" height="15" />
  )

  function selectPodIdInput(podId: string | null) {
    const input = document.getElementById(
      `pod-${row.values.podId}`
    ) as HTMLInputElement
    input && input.select()
  }

  if (!row.values.podId) return null
  return (
    <PodId>
      <PodIdInput
        id={`pod-${row.values.podId}`}
        value={row.values.podId}
        readOnly={true}
        onClick={() => selectPodIdInput(row.values.podId)}
      />
      <PodIdCopy
        onClick={copyClick}
        analyticsName="ui.web.overview.copyPodID"
        title="Copy Pod ID"
      >
        {icon}
      </PodIdCopy>
    </PodId>
  )
}

function TableEndpointColumn({ row }: CellProps<RowValues>) {
  let endpoints = row.original.endpoints.map((ep: any) => {
    return (
      <Endpoint
        onClick={() =>
          void incr("ui.web.endpoint", { action: AnalyticsAction.Click })
        }
        href={ep.url}
        // We use ep.url as the target, so that clicking the link re-uses the tab.
        target={ep.url}
        key={ep.url}
      >
        <StyledLinkSvg />
        <DetailText title={ep.name || displayURL(ep)}>
          {ep.name || displayURL(ep)}
        </DetailText>
      </Endpoint>
    )
  })
  return <>{endpoints}</>
}

function TableTriggerModeColumn({ row }: CellProps<RowValues>) {
  let isTiltfile = row.values.name == "(Tiltfile)"

  if (isTiltfile) return null
  return (
    <OverviewTableTriggerModeToggle
      resourceName={row.values.name}
      triggerMode={row.values.triggerMode}
    />
  )
}

function TableWidgetsColumn({ row }: CellProps<RowValues>) {
  const buttons = row.original.buttons.map((b: UIButton) => {
    let content = (
      <CustomActionButton key={b.metadata?.name} uiButton={b}>
        <ApiIcon
          iconName={b.spec?.iconName || "smart_button"}
          iconSVG={b.spec?.iconSVG}
        />
      </CustomActionButton>
    )

    if (b.spec?.text) {
      content = (
        <TiltTooltip title={b.spec.text}>
          <span>{content}</span>
        </TiltTooltip>
      )
    }

    return (
      <React.Fragment key={b.metadata?.name || ""}>{content}</React.Fragment>
    )
  })
  return <WidgetCell>{buttons}</WidgetCell>
}

function statusSortKey(row: RowValues): string {
  const status = row.statusLine
  let order
  if (
    status.buildStatus == ResourceStatus.Unhealthy ||
    status.runtimeStatus === ResourceStatus.Unhealthy
  ) {
    order = 0
  } else if (status.buildAlertCount || status.runtimeAlertCount) {
    order = 1
  } else {
    order = 2
  }
  // add name after order just to keep things stable when orders are equal
  return `${order}${row.name}`
}

// https://react-table.tanstack.com/docs/api/useTable#column-options
// The docs on this are not very clear!
// `accessor` should return a primitive, and that primitive is used for sorting and filtering
// the Cell function can get whatever it needs to render via row.original
// best evidence I've (Matt) found: https://github.com/tannerlinsley/react-table/discussions/2429#discussioncomment-25582
//   (from the author)
const columnDefs: Column<RowValues>[] = [
  {
    Header: () => <TableHeaderStarIcon title="Starred" />,
    id: "starred",
    accessor: "name", // Note: this accessor is meaningless but required when `Header` returns JSX.The starred column gets its data directly from the StarredResources context and sort on this column is disabled.
    disableSortBy: true,
    width: "10px",
    Cell: TableStarColumn,
  },
  {
    Header: "Updated",
    width: "25px",
    accessor: "lastDeployTime",
    Cell: TableUpdateColumn,
  },
  {
    Header: "Trigger",
    accessor: "trigger",
    disableSortBy: true,
    width: "20px",
    Cell: TableTriggerColumn,
  },
  {
    Header: "Resource Name",
    width: "280px",
    accessor: "name",
    Cell: TableNameColumn,
  },
  {
    Header: "Type",
    accessor: "resourceTypeLabel",
    width: "150px",
  },
  {
    Header: "Status",
    accessor: (row) => statusSortKey(row),
    width: "200px",
    Cell: TableStatusColumn,
  },
  {
    Header: "Pod ID",
    accessor: "podId",
    width: "50px",
    Cell: TablePodIDColumn,
  },
  {
    Header: "Widgets",
    id: "widgets",
    accessor: (row) => row.buttons.length,
    Cell: TableWidgetsColumn,
  },
  {
    Header: "Endpoints",
    id: "endpoints",
    accessor: (row) => row.endpoints.length,
    sortType: "basic",
    Cell: TableEndpointColumn,
  },
  {
    Header: "Trigger Mode",
    accessor: "triggerMode",
    width: "70px",
    Cell: TableTriggerModeColumn,
  },
]

const columnNameToInfoTooltip: {
  [key: string]: NonNullable<React.ReactNode>
} = {
  "Trigger Mode": (
    <>
      Trigger mode can be toggled through the UI. To set it persistently, see{" "}
      <a
        href={linkToTiltDocs(TiltDocsPage.TriggerMode)}
        target="_blank"
        rel="noopener noreferrer"
      >
        Tiltfile docs
      </a>
      .
    </>
  ),
  Widgets: (
    <>
      Buttons can be added to resources to easily perform custom actions. See{" "}
      <a
        href={linkToTiltDocs(TiltDocsPage.CustomButtons)}
        target="_blank"
        rel="noopener noreferrer"
      >
        buttons docs
      </a>
      .
    </>
  ),
}

function ResourceTableHeaderTip(props: { name?: string }) {
  if (!props.name) {
    return null
  }

  const tooltipContent = columnNameToInfoTooltip[props.name]
  if (!tooltipContent) {
    return null
  }

  return (
    <TiltInfoTooltip
      title={tooltipContent}
      dismissId={`table-header-${props.name}`}
    />
  )
}

const FIRST_SORT_STATE = false
const SECOND_SORT_STATE = true

// This helper function manually implements the toggle sorting
// logic used by react-table, so we can keep the sorting state
// globally and sort multiple tables by the same column.
//    Click once to sort by ascending values
//    Click twice to sort by descending values
//    Click thrice to remove sort
// Note: unlike react-table, this function does NOT support
// sorting by multiple columns right now.
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

async function copyTextToClipboard(text: string, cb: () => void) {
  await navigator.clipboard.writeText(text)
  cb()
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
  let buttons = buttonsForComponent(allButtons, "resource", r.metadata?.name)

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
    },
    podId: res.k8sResourceInfo?.podName ?? "",
    endpoints: res.endpointLinks ?? [],
    triggerMode: res.triggerMode ?? TriggerMode.TriggerModeAuto,
    buttons: buttons,
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

export function resourcesToTableCells(
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

export function TableHeadRow({
  headerGroup,
  setGlobalSortBy,
}: TableHeadRowProps) {
  const calculateToggleProps = (column: HeaderGroup<RowValues>) => {
    // Warning! Toggle props are not typed or documented well within react-table.
    // Modify toggle props with caution.
    // See https://react-table.tanstack.com/docs/api/useSortBy#column-properties
    const toggleProps: { [key: string]: any } = {
      title: column.canSort
        ? `Sort by ${column.render("Header")}`
        : column.render("Header"),
    }

    if (setGlobalSortBy) {
      // If the sort state is global, rather than individual to each table,
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
            <ResourceTableHeaderTip name={String(column.Header)} />
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

export function Table(props: TableProps) {
  const {
    getTableProps,
    getTableBodyProps,
    headerGroups,
    rows,
    prepareRow,
  } = useTable(
    {
      columns: columnDefs,
      data: props.data,
      autoResetSortBy: false,
      useControlledState: props.useControlledState,
    },
    useSortBy
  )

  const isGroupClass = props.isGroupView ? "isGroup" : ""

  // TODO (lizz): Consider adding `aria-sort` markup to table headings
  return (
    <ResourceTable {...getTableProps()} className={isGroupClass}>
      <ResourceTableHead>
        {headerGroups.map((headerGroup: HeaderGroup<RowValues>) => (
          <TableHeadRow
            {...headerGroup.getHeaderGroupProps()}
            headerGroup={headerGroup}
            setGlobalSortBy={props.setGlobalSortBy}
          />
        ))}
      </ResourceTableHead>
      <tbody {...getTableBodyProps()}>
        {rows.map((row: Row<RowValues>) => {
          prepareRow(row)
          return (
            <ResourceTableRow {...row.getRowProps()}>
              {row.cells.map((cell) => (
                <ResourceTableData
                  {...cell.getCellProps({
                    className: cell.column.isSorted ? "isSorted" : "",
                  })}
                >
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
          aria-label={`Status summary for ${label} group`}
          resources={tableProps.data}
        />
      </OverviewGroupSummary>
      <OverviewGroupDetails>
        <Table {...tableProps} isGroupView />
      </OverviewGroupDetails>
    </OverviewGroup>
  )
}

export function TableGroupedByLabels(props: OverviewTableProps) {
  const logAlertIndex = useLogAlertIndex()
  const data = useMemo(
    () =>
      resourcesToTableCells(
        props.view.uiResources,
        props.view.uiButtons,
        logAlertIndex
      ),
    [props.view.uiResources, props.view.uiButtons]
  )

  // Global table settings are currently used to sort multiple
  // tables by the same column
  // See: https://react-table.tanstack.com/docs/faq#how-can-i-manually-control-the-table-state
  const [globalTableSettings, setGlobalTableSettings] = useState<
    TableState<RowValues>
  >()

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
          columns={columnDefs}
          useControlledState={useControlledState}
          setGlobalSortBy={setGlobalSortBy}
        />
      ))}
      <TableGroup
        label={UNLABELED_LABEL}
        data={data.unlabeled}
        columns={columnDefs}
        useControlledState={useControlledState}
        setGlobalSortBy={setGlobalSortBy}
      />
      <TableGroup
        label={TILTFILE_LABEL}
        data={data.tiltfile}
        columns={columnDefs}
        useControlledState={useControlledState}
        setGlobalSortBy={setGlobalSortBy}
      />
    </>
  )
}

export function TableWithoutGroups(props: OverviewTableProps) {
  const logAlertIndex = useLogAlertIndex()
  const data = useMemo(() => {
    return (
      props.view.uiResources?.map((r) =>
        uiResourceToCell(r, props.view.uiButtons, logAlertIndex)
      ) || []
    )
  }, [props.view.uiResources, props.view.uiButtons])

  return <Table columns={columnDefs} data={data} />
}

export default function OverviewTable(props: OverviewTableProps) {
  const features = useFeatures()
  const labelsEnabled = features.isEnabled(Flag.Labels)
  const resourcesHaveLabels =
    props.view.uiResources?.some((r) => getResourceLabels(r).length > 0) ||
    false

  // The label group tip is only displayed if labels are enabled but not used
  const displayLabelGroupsTip = labelsEnabled && !resourcesHaveLabels

  if (labelsEnabled && resourcesHaveLabels) {
    return (
      <OverviewTableRoot aria-label="Resources overview">
        <TableGroupedByLabels {...props} />
      </OverviewTableRoot>
    )
  } else {
    return (
      <OverviewTableRoot aria-label="Resources overview">
        {displayLabelGroupsTip && (
          <ResourceGroupsInfoTip idForIcon={GROUP_INFO_TOOLTIP_ID} />
        )}
        <TableWithoutGroups
          aria-describedby={
            displayLabelGroupsTip ? GROUP_INFO_TOOLTIP_ID : undefined
          }
          {...props}
        />
      </OverviewTableRoot>
    )
  }
}
