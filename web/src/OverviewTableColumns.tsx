import React, { ChangeEvent, useCallback, useMemo, useState } from "react"
import { CellProps, Column, HeaderProps, Row } from "react-table"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { AnalyticsAction, AnalyticsType, incr, Tags } from "./analytics"
import { ApiButton, ApiIcon, ButtonSet } from "./ApiButton"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { ReactComponent as StarSvg } from "./assets/svg/star.svg"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import Features, { Flag } from "./feature"
import { Hold } from "./Hold"
import {
  InstrumentedButton,
  InstrumentedCheckbox,
} from "./instrumentedComponents"
import { displayURL } from "./links"
import { OverviewButtonMixin } from "./OverviewButton"
import { OverviewTableBuildButton } from "./OverviewTableBuildButton"
import OverviewTableStarResourceButton from "./OverviewTableStarResourceButton"
import OverviewTableStatus from "./OverviewTableStatus"
import OverviewTableTriggerModeToggle from "./OverviewTableTriggerModeToggle"
import { useResourceNav } from "./ResourceNav"
import { useResourceSelection } from "./ResourceSelectionContext"
import { disabledResourceStyleMixin } from "./ResourceStatus"
import { useStarredResources } from "./StarredResourcesContext"
import {
  Color,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import { timeAgoFormatter } from "./timeFormatters"
import TiltTooltip, { TiltInfoTooltip } from "./Tooltip"
import { startBuild } from "./trigger"
import { ResourceStatus, TriggerMode, UIButton, UILink } from "./types"

/**
 * Types
 */
type OverviewTableBuildButtonStatus = {
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

export type RowValues = {
  lastDeployTime: string
  trigger: OverviewTableBuildButtonStatus
  name: string
  resourceTypeLabel: string
  statusLine: OverviewTableResourceStatus
  podId: string
  endpoints: UILink[]
  triggerMode: TriggerMode
  buttons: ButtonSet
  analyticsTags: Tags
  selectable: boolean
}

/**
 * Styles
 */

export const SelectionCheckbox = styled(InstrumentedCheckbox)`
  &.MuiCheckbox-root,
  &.Mui-checked {
    color: ${Color.gray60};
  }

  &.Mui-disabled {
    opacity: 0.25;
    cursor: not-allowed;
  }
`

const TableHeaderStarIcon = styled(StarSvg)`
  fill: ${Color.gray70};
  height: 13px;
  width: 13px;
`

export const Name = styled.button`
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
    ${disabledResourceStyleMixin};
    color: ${Color.gray60};
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
  fill: ${Color.gray50};
  margin-right: ${SizeUnit(0.2)};
`

const PodId = styled.div`
  display: flex;
  align-items: center;
`
const PodIdInput = styled.input`
  background-color: transparent;
  color: ${Color.gray60};
  font-family: inherit;
  font-size: inherit;
  border: 1px solid ${Color.gray10};
  border-radius: 2px;
  padding: ${SizeUnit(0.1)} ${SizeUnit(0.2)};
  width: 100px;

  &::selection {
    background-color: ${Color.gray30};
  }
`
const PodIdCopy = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  padding-top: ${SizeUnit(0.5)};
  padding: ${SizeUnit(0.25)};

  svg {
    fill: ${Color.gray60};
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
    margin-top: ${SizeUnit(0.125)};
  }
`

/**
 * Table data helpers
 */

export function rowIsDisabled(row: Row<RowValues>): boolean {
  // If a resource is disabled, both runtime and build statuses should
  // be `disabled` and it won't matter which one we look at
  return row.original.statusLine.runtimeStatus === ResourceStatus.Disabled
}

async function copyTextToClipboard(text: string, cb: () => void) {
  await navigator.clipboard.writeText(text)
  cb()
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

/**
 * Header components
 */
export function ResourceSelectionHeader({
  rows,
  column,
}: HeaderProps<RowValues>) {
  const { selected, isSelected, select, deselect } = useResourceSelection()

  const selectableResourcesInTable = useMemo(() => {
    const resources: string[] = []
    rows.forEach(({ original }) => {
      if (original.selectable) {
        resources.push(original.name)
      }
    })

    return resources
  }, [rows])

  function getSelectionState(resourcesInTable: string[]): {
    indeterminate: boolean
    checked: boolean
  } {
    let anySelected = false
    let anyUnselected = false
    for (let i = 0; i < resourcesInTable.length; i++) {
      if (isSelected(resourcesInTable[i])) {
        anySelected = true
      } else {
        anyUnselected = true
      }

      if (anySelected && anyUnselected) {
        break
      }
    }

    return {
      indeterminate: anySelected && anyUnselected,
      checked: !anyUnselected,
    }
  }

  const { indeterminate, checked } = useMemo(
    () => getSelectionState(selectableResourcesInTable),
    [selectableResourcesInTable, selected]
  )

  // If no resources in the table are selectable, don't render
  if (selectableResourcesInTable.length === 0) {
    return null
  }

  const onChange = (_e: ChangeEvent<HTMLInputElement>) => {
    if (!checked) {
      select(...selectableResourcesInTable)
    } else {
      deselect(...selectableResourcesInTable)
    }
  }

  const analyticsTags: Tags = {
    type: AnalyticsType.Grid,
  }

  return (
    <SelectionCheckbox
      aria-label="Resource group selection"
      analyticsName={"ui.web.checkbox.resourceGroupSelection"}
      analyticsTags={analyticsTags}
      checked={checked}
      aria-checked={checked}
      indeterminate={indeterminate}
      onChange={onChange}
      size="small"
    />
  )
}

/**
 * Column components
 */
export function TableStarColumn({ row }: CellProps<RowValues>) {
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

export function TableUpdateColumn({ row }: CellProps<RowValues>) {
  if (!row.values.lastDeployTime) {
    return null
  }
  return (
    <TimeAgo date={row.values.lastDeployTime} formatter={timeAgoFormatter} />
  )
}

export function TableSelectionColumn({ row }: CellProps<RowValues>) {
  const selections = useResourceSelection()
  const resourceName = row.original.name
  const checked = selections.isSelected(resourceName)

  const onChange = useCallback(
    (_e: ChangeEvent<HTMLInputElement>) => {
      if (!checked) {
        selections.select(resourceName)
      } else {
        selections.deselect(resourceName)
      }
    },
    [checked, selections]
  )

  const analyticsTags = useMemo(() => {
    return {
      ...row.original.analyticsTags,
      type: AnalyticsType.Grid,
    }
  }, [row.original.analyticsTags])

  let disabled = !row.original.selectable
  let label = row.original.selectable
    ? "Select resource"
    : "Cannot select resource"

  return (
    <SelectionCheckbox
      analyticsName={"ui.web.checkbox.resourceSelection"}
      analyticsTags={analyticsTags}
      checked={checked}
      aria-checked={checked}
      onChange={onChange}
      size="small"
      disabled={disabled}
      aria-label={label}
    />
  )
}

export function TableBuildButtonColumn({ row }: CellProps<RowValues>) {
  // If resource is disabled, don't display build button
  if (rowIsDisabled(row)) {
    return null
  }

  const trigger = row.original.trigger
  let onStartBuild = useCallback(
    () => startBuild(row.values.name),
    [row.values.name]
  )
  return (
    <OverviewTableBuildButton
      hasPendingChanges={trigger.hasPendingChanges}
      hasBuilt={trigger.hasBuilt}
      isBuilding={trigger.isBuilding}
      triggerMode={row.values.triggerMode}
      isQueued={trigger.isQueued}
      analyticsTags={row.values.analyticsTags}
      onStartBuild={onStartBuild}
      stopBuildButton={row.original.buttons.stopBuild}
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

let TableStatusColumnRoot = styled.div`
  display: flex;
  flex-direction: column;
  align-items: start;
  justify-content: space-around;
  min-height: 4em;
`

export function TableStatusColumn({ row }: CellProps<RowValues>) {
  const status = row.original.statusLine
  const runtimeStatus = (
    <OverviewTableStatus
      status={status.runtimeStatus}
      resourceName={row.values.name}
    />
  )

  // If a resource is disabled, only one status needs to be displayed
  if (rowIsDisabled(row)) {
    return <TableStatusColumnRoot>{runtimeStatus}</TableStatusColumnRoot>
  }

  return (
    <TableStatusColumnRoot>
      <OverviewTableStatus
        status={status.buildStatus}
        lastBuildDur={status.lastBuildDur}
        isBuild={true}
        resourceName={row.values.name}
        hold={status.hold}
      />
      {runtimeStatus}
    </TableStatusColumnRoot>
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

export function TableWidgetsColumn({ row }: CellProps<RowValues>) {
  // If a resource is disabled, don't display any buttons
  if (rowIsDisabled(row)) {
    return null
  }

  const buttons = row.original.buttons.default.map((b: UIButton) => {
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

/**
 * Column tooltips
 */

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

export function ResourceTableHeaderTip(props: { name?: string }) {
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

/**
 * Column definitions
 */
const RESOURCE_SELECTION_COLUMN: Column<RowValues> = {
  Header: (props) => <ResourceSelectionHeader {...props} />,
  id: "selection",
  disableSortBy: true,
  width: "70px",
  Cell: TableSelectionColumn,
}

// https://react-table.tanstack.com/docs/api/useTable#column-options
// The docs on this are not very clear!
// `accessor` should return a primitive, and that primitive is used for sorting and filtering
// the Cell function can get whatever it needs to render via row.original
// best evidence I've (Matt) found: https://github.com/tannerlinsley/react-table/discussions/2429#discussioncomment-25582
//   (from the author)
const DEFAULT_COLUMNS: Column<RowValues>[] = [
  {
    Header: () => <TableHeaderStarIcon title="Starred" />,
    id: "starred",
    disableSortBy: true,
    width: "40px",
    Cell: TableStarColumn,
  },
  {
    Header: "Updated",
    accessor: "lastDeployTime",
    width: "100px",
    Cell: TableUpdateColumn,
  },
  {
    Header: "Trigger",
    accessor: "trigger",
    disableSortBy: true,
    Cell: TableBuildButtonColumn,
    width: "80px",
  },
  {
    Header: "Resource Name",
    accessor: "name",
    Cell: TableNameColumn,
    width: "400px",
  },
  {
    Header: "Type",
    accessor: "resourceTypeLabel",
    width: "auto",
  },
  {
    Header: "Status",
    accessor: (row) => statusSortKey(row),
    Cell: TablePodIDColumn,
    width: "auto",
  },
  {
    Header: "Pod ID",
    accessor: "podId",
    width: "auto",
    Cell: TablePodIDColumn,
  },
  {
    Header: "Widgets",
    id: "widgets",
    accessor: (row) => row.buttons.default.length,
    Cell: TableWidgetsColumn,
    width: "auto",
  },
  {
    Header: "Endpoints",
    id: "endpoints",
    accessor: (row) => row.endpoints.length,
    sortType: "basic",
    Cell: TableEndpointColumn,
    width: "auto",
  },
  {
    Header: "Mode",
    accessor: "triggerMode",
    Cell: TableTriggerModeColumn,
    width: "auto",
  },
]

let ALL_COLUMNS = [RESOURCE_SELECTION_COLUMN, ...DEFAULT_COLUMNS]

export function getTableColumns(features?: Features) {
  if (!features) {
    return DEFAULT_COLUMNS
  }

  // If disable resources is enabled, render the selection column
  if (features.isEnabled(Flag.DisableResources)) {
    return ALL_COLUMNS
  }

  return DEFAULT_COLUMNS
}
