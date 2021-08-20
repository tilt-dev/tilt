import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import React, { ChangeEvent, useMemo, useState } from "react"
import {
  CellProps,
  Column,
  TableOptions,
  useSortBy,
  useTable,
} from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { buildAlerts, runtimeAlerts } from "./alerts"
import { AnalyticsAction, AnalyticsType, incr } from "./analytics"
import { ApiIcon, buttonsForComponent } from "./ApiButton"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import { InstrumentedButton } from "./instrumentedComponents"
import {
  getResourceLabels,
  GroupByLabelView,
  orderLabels,
  TILTFILE_LABEL,
  UNLABELED_LABEL,
} from "./labels"
import { displayURL } from "./links"
import LogStore, { LogAlertIndex, useLogStore } from "./LogStore"
import { CustomActionButton } from "./OverviewButton"
import OverviewTableStarResourceButton from "./OverviewTableStarResourceButton"
import OverviewTableStatus from "./OverviewTableStatus"
import OverviewTableTriggerButton from "./OverviewTableTriggerButton"
import OverviewTableTriggerModeToggle from "./OverviewTableTriggerModeToggle"
import {
  AccordionDetailsStyleResetMixin,
  AccordionStyleResetMixin,
  AccordionSummaryStyleResetMixin,
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
`

const ResourceTable = styled.table`
  margin-top: ${SizeUnit(0.5)};
  border-collapse: collapse;
  width: 100%;

  td:first-child {
    padding-left: ${SizeUnit(1)};
  }
  td:last-child {
    padding-right: ${SizeUnit(1)};
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
const ResourceTableData = styled.td`
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
  color: ${Color.gray6};
  box-sizing: border-box;
`
const ResourceTableHeader = styled(ResourceTableData)`
  color: ${Color.gray7};
  font-size: ${FontSize.smallest};
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  box-sizing: border-box;
  white-space: nowrap;
`

const ResourceTableHeaderLabel = styled.div`
  display: flex;
  align-items: center;
`

const ResourceTableHeaderSortTriangle = styled.div`
  display: inline-block;
  margin-left: ${SizeUnit(0.25)};
  width: 0;
  height: 0;
  border-left: 4px solid transparent;
  border-right: 4px solid transparent;
  border-bottom: 6px solid ${Color.grayLight};

  &.is-sorted-asc {
    border-bottom: 6px solid ${Color.blue};
  }
  &.is-sorted-desc {
    border-bottom: 6px solid ${Color.blue};
    transform: rotate(180deg);
  }
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
const WidgetCell = styled.span`
  display: flex;
  flex-wrap: wrap;
  max-width: ${SizeUnit(8)};

  ${CustomActionButton} {
    margin-bottom: ${SizeUnit(0.125)};
    margin-right: ${SizeUnit(0.125)};
  }
`

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
  return (
    <TimeAgo date={row.values.lastDeployTime} formatter={timeAgoFormatter} />
  )
}

function TableTriggerColumn({ row }: CellProps<RowValues>) {
  return (
    <OverviewTableTriggerButton
      hasPendingChanges={row.values.trigger.hasPendingChanges}
      hasBuilt={row.values.trigger.hasBuilt}
      isBuilding={row.values.trigger.isBuilding}
      triggerMode={row.values.triggerMode}
      isQueued={row.values.trigger.isQueued}
      resourceName={row.values.name}
    />
  )
}

export function TableNameColumn({ row }: CellProps<RowValues>) {
  let nav = useResourceNav()
  let hasError =
    row.values.statusLine.buildStatus === ResourceStatus.Unhealthy ||
    row.values.statusLine.runtimeStatus === ResourceStatus.Unhealthy

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
  return (
    <>
      <OverviewTableStatus
        status={row.values.statusLine.buildStatus}
        lastBuildDur={row.values.statusLine.lastBuildDur}
        alertCount={row.values.statusLine.buildAlertCount}
        isBuild={true}
        resourceName={row.values.name}
      />
      <OverviewTableStatus
        status={row.values.statusLine.runtimeStatus}
        alertCount={row.values.statusLine.runtimeAlertCount}
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
  let endpoints = row.values.endpoints.map((ep: any) => {
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
      <CustomActionButton key={b.metadata?.name} button={b}>
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

// https://react-table.tanstack.com/docs/api/useTable#column-options
// The docs on this are not very clear!
// `accessor` should return a primitive, and that primitive is used for sorting and filtering
// the Cell function can get whatever it needs to render via row.original
// best evidence I've (Matt) found: https://github.com/tannerlinsley/react-table/discussions/2429#discussioncomment-25582
//   (from the author)
// TODO: fix existing columns to return reasonable primitives from `accessor`
const columnDefs: Column<RowValues>[] = [
  {
    Header: "Starred",
    width: "20px",
    Cell: TableStarColumn,
  },
  {
    Header: "Updated",
    width: "20px",
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
    accessor: "statusLine",
    disableSortBy: true,
    width: "150px",
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
    accessor: "endpoints",
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
}

function ResourceTableHeaderTip(props: { name?: string }) {
  if (!props.name) {
    return null
  }

  const tooltipContent = columnNameToInfoTooltip[props.name]
  if (!tooltipContent) {
    return null
  }

  return <TiltInfoTooltip title={tooltipContent} />
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
      lastBuildDur: lastBuildDur,
      buildAlertCount: buildAlerts(r, alertIndex).length,
      runtimeAlertCount: runtimeAlerts(r, alertIndex).length,
      runtimeStatus: runtimeStatus(r, alertIndex),
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

function hasWidgets(view: Proto.webviewView): boolean {
  return !!view.uiButtons?.length
}

function viewToRowValues(
  view: Proto.webviewView,
  logStore: LogStore
): RowValues[] {
  return (
    view.uiResources?.map((r) =>
      uiResourceToCell(r, view.uiButtons, logStore)
    ) || []
  )
}

export function resourcesToTableCells(
  resources: UIResource[] | undefined,
  buttons: UIButton[] | undefined,
  logStore: LogStore
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
    const tableCell = uiResourceToCell(r, buttons, logStore)
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

export function Table(
  props: TableOptions<RowValues> & { isGroupView?: boolean }
) {
  const {
    getTableProps,
    getTableBodyProps,
    headerGroups,
    rows,
    prepareRow,
    columns,
  } = useTable(
    {
      columns: columnDefs,
      data: props.data,
      autoResetSortBy: false,
    },
    useSortBy
  )

  const isGroupClass = props.isGroupView ? "isGroup" : ""

  return (
    <ResourceTable {...getTableProps()} className={isGroupClass}>
      <ResourceTableHead>
        {headerGroups.map((headerGroup) => (
          <ResourceTableRow {...headerGroup.getHeaderGroupProps()}>
            {headerGroup.headers.map((column) => (
              <ResourceTableHeader
                {...column.getHeaderProps([
                  { style: { width: column.width } },
                  column.getSortByToggleProps({
                    title: `Sort by ${column.render("Header")}`,
                  }),
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

function TableGroup(props: { label: string; data: RowValues[] }) {
  if (props.data.length === 0) {
    return null
  }

  const formattedLabel =
    props.label === UNLABELED_LABEL ? <em>{props.label}</em> : props.label
  const labelNameId = `tableOverview-${props.label}`

  const { getGroup, toggleGroupExpanded } = useResourceGroups()
  const { expanded } = getGroup(props.label)
  const handleChange = (_e: ChangeEvent<{}>) =>
    toggleGroupExpanded(props.label, AnalyticsType.Grid)

  return (
    <OverviewGroup expanded={expanded} onChange={handleChange}>
      <OverviewGroupSummary id={labelNameId}>
        <ResourceGroupSummaryIcon role="presentation" />
        <OverviewGroupName>{formattedLabel}</OverviewGroupName>
        <TableGroupStatusSummary
          aria-label={`Status summary for ${props.label} group`}
          resources={props.data}
        />
      </OverviewGroupSummary>
      <OverviewGroupDetails>
        <Table columns={columnDefs} data={props.data} isGroupView />
      </OverviewGroupDetails>
    </OverviewGroup>
  )
}

export function TableGroupedByLabels(props: OverviewTableProps) {
  const logStore = useLogStore()
  const data = useMemo(
    () =>
      resourcesToTableCells(
        props.view.uiResources,
        props.view.uiButtons,
        logStore
      ),
    [props.view.uiResources, props.view.uiButtons]
  )
  return (
    <>
      {data.labels.map((label) => (
        <TableGroup
          key={label}
          label={label}
          data={data.labelsToResources[label]}
        />
      ))}
      <TableGroup label={UNLABELED_LABEL} data={data.unlabeled} />
      <TableGroup label={TILTFILE_LABEL} data={data.tiltfile} />
    </>
  )
}

function TableWithoutGroups(props: OverviewTableProps) {
  const logStore = useLogStore()
  const data = useMemo(() => {
    return (
      props.view.uiResources?.map((r) =>
        uiResourceToCell(r, props.view.uiButtons, logStore)
      ) || []
    )
  }, [props.view.uiResources, props.view.uiButtons])

  return <Table columns={columnDefs} data={data} />
}

export default function OverviewTable(props: OverviewTableProps) {
  // TODO (lizz): Add support for table groups by feature flag
  // when groups are ready to launch
  return <TableWithoutGroups {...props} />
}
