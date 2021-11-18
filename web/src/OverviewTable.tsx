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
import { AnalyticsAction, AnalyticsType, incr, Tags } from "./analytics"
import { ApiButton, ApiIcon, buttonsForComponent } from "./ApiButton"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import Features, { Flag, useFeatures } from "./feature"
import { Hold } from "./Hold"
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
import {
  ResourceListOptions,
  useResourceListOptions,
} from "./ResourceListOptionsContext"
import { matchesResourceName, ResourceNameFilter } from "./ResourceNameFilter"
import { useResourceNav } from "./ResourceNav"
import {
  disabledResourceStyleMixin,
  resourceIsDisabled,
  resourceTargetType,
} from "./ResourceStatus"
import { TableGroupStatusSummary } from "./ResourceStatusSummary"
import { useStarredResources } from "./StarredResourcesContext"
import { buildStatus, runtimeStatus } from "./status"
import {
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
  Width,
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

type TableWrapperProps = {
  resources?: UIResource[]
  buttons?: UIButton[]
}

type TableGroupProps = {
  label: string
  setGlobalSortBy: (id: string) => void
} & TableOptions<RowValues>

type TableProps = {
  setGlobalSortBy?: (id: string) => void
} & TableOptions<RowValues>

type ResourceTableHeadRowProps = {
  headerGroup: HeaderGroup<RowValues>
  setGlobalSortBy?: (id: string) => void
} & TableHeaderProps

export type RowValues = {
  lastDeployTime: string
  trigger: OverviewTableTrigger
  name: string
  resourceTypeLabel: string
  statusLine: OverviewTableResourceStatus
  podId: string
  endpoints: UILink[]
  triggerMode: TriggerMode
  buttons: UIButton[]
  analyticsTags: Tags
}

type OverviewTableTrigger = {
  isBuilding: boolean
  hasBuilt: boolean
  hasPendingChanges: boolean
  isQueued: boolean
}

type OverviewTableResourceStatus = {
  buildStatus: ResourceStatus
  buildAlertCount: number
  lastBuildDur: moment.Duration | null
  runtimeStatus: ResourceStatus
  runtimeAlertCount: number
  hold?: Hold | null
}

// Resource name filter styles
const OverviewTableResourceNameFilter = styled(ResourceNameFilter)`
  margin-right: ${SizeUnit(1 / 2)};
  min-width: ${Width.sidebarDefault}px;
`

const ResourceResultCount = styled.p`
  color: ${Color.grayLight};
  font-size: ${FontSize.small};
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(0.5)};
  text-transform: uppercase;
`

const NoMatchesFound = styled.p`
  color: ${Color.grayLightest};
  margin-left: ${SizeUnit(0.5)};
  margin-top: ${SizeUnit(1 / 4)};
`

// Table styles
const OverviewTableRoot = styled.section`
  margin-bottom: ${SizeUnit(1 / 2)};
  margin-left: ${SizeUnit(1 / 2)};
  margin-right: ${SizeUnit(1 / 2)};
`

const ResourceTable = styled.table`
  margin-top: ${SizeUnit(0.5)};
  border-collapse: collapse;
  border: 1px ${Color.grayLighter} solid;
  border-radius: 0 ${SizeUnit(1 / 4)};
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
`
const ResourceTableHead = styled.thead`
  background-color: ${Color.grayDarker};
`
export const ResourceTableRow = styled.tr`
  border-bottom: 1px solid ${Color.grayLighter};
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
  font-style: none;
  color: ${Color.gray6};

  &.isDisabled {
    ${disabledResourceStyleMixin}
  }
`
export const ResourceTableData = styled.td`
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

  &.isDisabled {
    ${disabledResourceStyleMixin}
    color: ${Color.gray6};
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

  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded {
    margin-top: ${SizeUnit(1 / 2)};
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

function TableStarColumn({ row }: CellProps<RowValues>) {
  let ctx = useStarredResources()
  return (
    <OverviewTableStarResourceButton
      resourceName={row.values.name}
      analyticsName="ui.web.overviewStarButton"
      analyticsTags={row.values.analyticsTags}
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

export function TableTriggerColumn({ row }: CellProps<RowValues>) {
  // If resource is disabled, don't display trigger button
  if (rowIsDisabled(row)) {
    return null
  }

  const trigger = row.original.trigger
  return (
    <OverviewTableTriggerButton
      hasPendingChanges={trigger.hasPendingChanges}
      hasBuilt={trigger.hasBuilt}
      isBuilding={trigger.isBuilding}
      triggerMode={row.values.triggerMode}
      isQueued={trigger.isQueued}
      resourceName={row.values.name}
      analyticsTags={row.values.analyticsTags}
    />
  )
}

export function TableNameColumn({ row }: CellProps<RowValues>) {
  let nav = useResourceNav()
  let hasError =
    row.original.statusLine.buildStatus === ResourceStatus.Unhealthy ||
    row.original.statusLine.runtimeStatus === ResourceStatus.Unhealthy
  const errorClass = hasError ? "has-error" : ""
  const disabledClass = rowIsDisabled(row) ? "isDisabled" : ""
  return (
    <Name
      className={`${errorClass} ${disabledClass}`}
      onClick={(e) => nav.openResource(row.values.name)}
    >
      {row.values.name}
    </Name>
  )
}

function TableStatusColumn({ row }: CellProps<RowValues>) {
  const status = row.original.statusLine
  const runtimeStatus = (
    <OverviewTableStatus
      status={status.runtimeStatus}
      resourceName={row.values.name}
    />
  )

  // If a resource is disabled, only one status needs to be displayed
  if (rowIsDisabled(row)) {
    return runtimeStatus
  }

  return (
    <>
      <OverviewTableStatus
        status={status.buildStatus}
        lastBuildDur={status.lastBuildDur}
        isBuild={true}
        resourceName={row.values.name}
        hold={status.hold}
      />
      {runtimeStatus}
    </>
  )
}

export function TablePodIDColumn({ row }: CellProps<RowValues>) {
  let [showCopySuccess, setShowCopySuccess] = useState(false)

  let copyClick = () => {
    copyTextToClipboard(row.values.podId, () => {
      setShowCopySuccess(true)

      setTimeout(() => {
        setShowCopySuccess(false)
      }, 3000)
    })
  }

  // If resource is disabled, don't display pod information
  if (rowIsDisabled(row)) {
    return null
  }

  let icon = showCopySuccess ? (
    <CheckmarkSvg width="15" height="15" />
  ) : (
    <CopySvg width="15" height="15" />
  )

  function selectPodIdInput() {
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
        onClick={() => selectPodIdInput()}
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

export function TableEndpointColumn({ row }: CellProps<RowValues>) {
  // If a resource is disabled, don't display any endpoints
  if (rowIsDisabled(row)) {
    return null
  }

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

export function TableTriggerModeColumn({ row }: CellProps<RowValues>) {
  let isTiltfile = row.values.name == "(Tiltfile)"
  const isDisabled = rowIsDisabled(row)

  if (isTiltfile || isDisabled) return null
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

function rowIsDisabled(row: Row<RowValues>): boolean {
  // If a resource is disabled, both runtime and build statuses should
  // be `disabled` and it won't matter which one we look at
  return row.original.statusLine.runtimeStatus === ResourceStatus.Disabled
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
  } else if (
    status.runtimeStatus === ResourceStatus.Disabled ||
    status.buildStatus === ResourceStatus.Disabled
  ) {
    // Disabled resources should appear last
    order = 3
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

async function copyTextToClipboard(text: string, cb: () => void) {
  await navigator.clipboard.writeText(text)
  cb()
}

function applyOptionsToResources(
  resources: UIResource[] | undefined,
  options: ResourceListOptions
): UIResource[] {
  if (!resources) {
    return []
  }

  if (options.resourceNameFilter.length === 0) {
    return resources
  }

  return resources.filter((r) =>
    matchesResourceName(r.metadata?.name || "", options.resourceNameFilter)
  )
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
  let analyticsTags = { target: resourceTargetType(r) }

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

function resourceListByStatus(
  resources: UIResource[] = [],
  features: Features
) {
  // If disabling resources feature is enabled, then sort by disabled status,
  // so disabled resources appear at the end of each table list.
  // Note: this initial sort is done here so it doesn't interfere with the sorting
  // managed by react-table
  if (features.isEnabled(Flag.DisableResources)) {
    const sorted = [...resources].sort((a, b) => {
      const resourceAOrder = resourceIsDisabled(a) ? 1 : 0
      const resourceBOrder = resourceIsDisabled(b) ? 1 : 0

      return resourceAOrder - resourceBOrder
    })
    return sorted
  } else {
    // If disabling resources feature is NOT enabled, then filter
    // out disabled resources from display
    return resources.filter((r) => !resourceIsDisabled(r))
  }
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
    const columnHeader =
      typeof column.Header === "string" ? column.Header : column.id

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
  if (props.data.length === 0) {
    return null
  }

  const { getTableProps, getTableBodyProps, headerGroups, rows, prepareRow } =
    useTable(
      {
        columns: columnDefs,
        data: props.data,
        autoResetSortBy: false,
        useControlledState: props.useControlledState,
      },
      useSortBy
    )

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
        {rows.map((row: Row<RowValues>) => {
          prepareRow(row)
          return (
            <ResourceTableRow
              {...row.getRowProps({
                className: rowIsDisabled(row) ? "isDisabled" : "",
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
        <Table {...tableProps} />
      </OverviewGroupDetails>
    </OverviewGroup>
  )
}

export function TableGroupedByLabels({
  resources,
  buttons,
}: TableWrapperProps) {
  const logAlertIndex = useLogAlertIndex()
  const data = useMemo(
    () => labeledResourcesToTableCells(resources, buttons, logAlertIndex),
    [resources, buttons]
  )

  // Global table settings are currently used to sort multiple
  // tables by the same column
  // See: https://react-table.tanstack.com/docs/faq#how-can-i-manually-control-the-table-state
  const [globalTableSettings, setGlobalTableSettings] =
    useState<TableState<RowValues>>()

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

export function TableWithoutGroups({ resources, buttons }: TableWrapperProps) {
  const logAlertIndex = useLogAlertIndex()
  const data = useMemo(() => {
    return (
      resources?.map((r) => uiResourceToCell(r, buttons, logAlertIndex)) || []
    )
  }, [resources, buttons])

  return <Table columns={columnDefs} data={data} />
}

function OverviewTableContent(props: OverviewTableProps) {
  const features = useFeatures()
  const labelsEnabled = features.isEnabled(Flag.Labels)
  const resourcesHaveLabels =
    props.view.uiResources?.some((r) => getResourceLabels(r).length > 0) ||
    false

  const { options } = useResourceListOptions()
  const resourceFilterApplied = options.resourceNameFilter.length > 0

  // Adjust the resource list based on feature flags
  const resourceList = resourceListByStatus(props.view.uiResources, features)

  // Table groups are displayed when feature is enabled, resources have labels,
  // and no resource name filter is applied
  const displayResourceGroups =
    labelsEnabled && resourcesHaveLabels && !resourceFilterApplied

  if (displayResourceGroups) {
    return (
      <TableGroupedByLabels
        resources={resourceList}
        buttons={props.view.uiButtons}
      />
    )
  } else {
    // The label group tip is only displayed if labels are enabled but not used
    const displayLabelGroupsTip = labelsEnabled && !resourcesHaveLabels

    // Apply any display filters or options to resources
    const resourcesToDisplay = applyOptionsToResources(resourceList, options)

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
      <OverviewTableResourceNameFilter />
      <OverviewTableContent {...props} />
    </OverviewTableRoot>
  )
}
