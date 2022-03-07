import { debounce, InputAdornment, InputProps } from "@material-ui/core"
import Menu from "@material-ui/core/Menu"
import MenuItem from "@material-ui/core/MenuItem"
import { PopoverOrigin } from "@material-ui/core/Popover"
import { makeStyles } from "@material-ui/core/styles"
import ExpandMoreIcon from "@material-ui/icons/ExpandMore"
import { History } from "history"
import React, { ChangeEvent, useEffect, useState } from "react"
import { useHistory, useLocation } from "react-router"
import styled from "styled-components"
import { Alert } from "./alerts"
import { AnalyticsAction, incr } from "./analytics"
import { ApiButton, ButtonSet } from "./ApiButton"
import { ReactComponent as AlertSvg } from "./assets/svg/alert.svg"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as FilterSvg } from "./assets/svg/filter.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import {
  InstrumentedButton,
  InstrumentedTextField,
} from "./instrumentedComponents"
import { displayURL } from "./links"
import LogActions from "./LogActions"
import {
  EMPTY_TERM,
  FilterLevel,
  FilterSet,
  FilterSource,
  FilterTerm,
  isErrorTerm,
  TermState,
} from "./logfilters"
import { useLogStore } from "./LogStore"
import OverviewActionBarKeyboardShortcuts from "./OverviewActionBarKeyboardShortcuts"
import { OverviewButtonMixin } from "./OverviewButton"
import { usePathBuilder } from "./PathBuilder"
import { resourceIsDisabled } from "./ResourceStatus"
import SrOnly from "./SrOnly"
import {
  AnimDuration,
  Color,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import { TiltInfoTooltip } from "./Tooltip"
import { ResourceName } from "./types"

type UIResource = Proto.v1alpha1UIResource
type Link = Proto.v1alpha1UIResourceLink
type UIButton = Proto.v1alpha1UIButton

type OverviewActionBarProps = {
  // The current resource. May be null if there is no resource.
  resource?: UIResource

  // All the alerts for the current resource.
  alerts?: Alert[]

  // The current log filter.
  filterSet: FilterSet

  // buttons for this resource
  buttons?: ButtonSet
}

type FilterSourceMenuProps = {
  id: string
  open: boolean
  anchorEl: Element | null
  onClose: () => void

  // The level button that this menu belongs to.
  level: FilterLevel

  // The current filter set.
  filterSet: FilterSet

  // The alerts for the current resource.
  alerts?: Alert[]
}

let useMenuStyles = makeStyles((theme) => ({
  root: {
    fontFamily: Font.sansSerif,
    fontSize: FontSize.smallest,
  },
}))

// Menu to filter logs by source (e.g., build-only, runtime-only).
function FilterSourceMenu(props: FilterSourceMenuProps) {
  let { id, anchorEl, level, open, onClose } = props
  let alerts = props.alerts || []

  let classes = useMenuStyles()
  let history = useHistory()
  let l = useLocation()
  let onClick = (e: any) => {
    let source = e.currentTarget.getAttribute("data-filter")
    const search = createLogSearch(l.search, { source, level })
    history.push({
      pathname: l.pathname,
      search: search.toString(),
    })
    onClose()
  }

  let anchorOrigin: PopoverOrigin = {
    vertical: "bottom",
    horizontal: "right",
  }
  let transformOrigin: PopoverOrigin = {
    vertical: "top",
    horizontal: "right",
  }

  let allCount: null | number = null
  let buildCount: null | number = null
  let runtimeCount: null | number = null
  if (level != FilterLevel.all) {
    allCount = alerts.reduce(
      (acc, alert) => (alert.level == level ? acc + 1 : acc),
      0
    )
    buildCount = alerts.reduce(
      (acc, alert) =>
        alert.level == level && alert.source == FilterSource.build
          ? acc + 1
          : acc,
      0
    )
    runtimeCount = alerts.reduce(
      (acc, alert) =>
        alert.level == level && alert.source == FilterSource.runtime
          ? acc + 1
          : acc,
      0
    )
  }
  return (
    <Menu
      id={id}
      anchorEl={anchorEl}
      open={open}
      onClose={onClose}
      disableScrollLock={true}
      keepMounted={true}
      anchorOrigin={anchorOrigin}
      transformOrigin={transformOrigin}
      getContentAnchorEl={null}
    >
      <MenuItem
        data-filter={FilterSource.all}
        classes={classes}
        onClick={onClick}
      >
        All Sources{allCount === null ? "" : ` (${allCount})`}
      </MenuItem>
      <MenuItem
        data-filter={FilterSource.build}
        classes={classes}
        onClick={onClick}
      >
        Build Only{buildCount === null ? "" : ` (${buildCount})`}
      </MenuItem>
      <MenuItem
        data-filter={FilterSource.runtime}
        classes={classes}
        onClick={onClick}
      >
        Runtime Only{runtimeCount === null ? "" : ` (${runtimeCount})`}
      </MenuItem>
    </Menu>
  )
}

const CustomActionButton = styled(ApiButton)`
  button {
    ${OverviewButtonMixin};
  }

  & + & {
    margin-left: ${SizeUnit(0.25)};
  }
`

const DisableButton = styled(ApiButton)`
  margin-right: ${SizeUnit(0.5)};

  button {
    ${OverviewButtonMixin};
    background-color: ${Color.gray20};

    &:hover {
      background-color: ${Color.gray20};
    }
  }

  button:first-child {
    width: 100%;
  }

  // hardcode a width to workaround this bug:
  // https://app.shortcut.com/windmill/story/12912/uibuttons-created-by-togglebuttons-have-different-sizes-when-toggled
  width: ${SizeUnit(4.4)};
`

const ButtonRoot = styled(InstrumentedButton)`
  ${OverviewButtonMixin}
`

const WidgetRoot = styled.div`
  display: flex;
  ${ButtonRoot} + ${ButtonRoot} {
    margin-left: ${SizeUnit(0.125)};
  }
`

let ButtonPill = styled.div`
  display: flex;
  margin-right: ${SizeUnit(0.5)};

  &.isCentered {
    margin-left: auto;
  }
`

export let ButtonLeftPill = styled(ButtonRoot)`
  border-radius: 4px 0 0 4px;
  border-right: 0;

  &:hover + button {
    border-left-color: ${Color.blue};
  }
`
export let ButtonRightPill = styled(ButtonRoot)`
  border-radius: 0 4px 4px 0;
`

const FilterTermTextField = styled(InstrumentedTextField)`
  & .MuiOutlinedInput-root {
    background-color: ${Color.gray20};
    position: relative;
    width: ${SizeUnit(9)};

    & fieldset {
      border: 1px solid ${Color.gray40};
      border-radius: ${SizeUnit(0.125)};
      transition: border-color ${AnimDuration.default} ease;
    }
    &:hover:not(.Mui-focused, .Mui-error) fieldset {
      border: 1px solid ${Color.blue};
    }
    &.Mui-focused fieldset {
      border: 1px solid ${Color.grayLightest};
    }
    &.Mui-error fieldset {
      border: 1px solid ${Color.red};
    }
    & .MuiOutlinedInput-input {
      padding: ${SizeUnit(0.2)};
    }
  }

  & .MuiInputBase-input {
    color: ${Color.gray70};
    font-family: ${Font.monospace};
    font-size: ${FontSize.small};
  }
`

const FieldErrorTooltip = styled.span`
  align-items: center;
  background-color: ${Color.gray20};
  box-sizing: border-box;
  color: ${Color.red};
  display: flex;
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
  left: 0;
  line-height: 1.4;
  margin: ${SizeUnit(0.25)} 0 0 0;
  padding: ${SizeUnit(0.25)};
  position: absolute;
  right: 0;
  top: 100%;
  z-index: 1;

  ::before {
    border-bottom: 8px solid ${Color.gray20};
    border-left: 8px solid transparent;
    border-right: 8px solid transparent;
    content: "";
    height: 0;
    left: 20px;
    position: absolute;
    top: -8px;
    width: 0;
  }
`

const AlertIcon = styled(AlertSvg)`
  padding-right: ${SizeUnit(0.25)};
`

const ClearFilterTermTextButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle}
  align-items: center;
  display: flex;
`

type FilterRadioButtonProps = {
  // The level that this button toggles.
  level: FilterLevel

  // The current filter set.
  filterSet: FilterSet

  // All the alerts for the current resource.
  alerts?: Alert[]

  className?: string
}

export function createLogSearch(
  currentSearch: string,
  {
    level,
    source,
    term,
  }: { level?: FilterLevel; source?: FilterSource; term?: string }
) {
  // Start with the existing search params
  const newSearch = new URLSearchParams(currentSearch)

  if (level !== undefined) {
    newSearch.set("level", level)
  }

  if (source !== undefined) {
    newSearch.set("source", source)
  }

  if (term !== undefined) {
    newSearch.set("term", term)
  }

  return newSearch
}

export function FilterRadioButton(props: FilterRadioButtonProps) {
  let { level, filterSet } = props
  let alerts = props.alerts || []
  let leftText = "All Levels"
  let count = alerts.reduce(
    (acc, alert) => (alert.level == level ? acc + 1 : acc),
    0
  )
  if (level === FilterLevel.warn) {
    leftText = `Warnings (${count})`
  } else if (level === FilterLevel.error) {
    leftText = `Errors (${count})`
  }

  let isEnabled = level === props.filterSet.level
  let rightText = (
    <ExpandMoreIcon
      style={{ width: "16px", height: "16px" }}
      key="right-text"
    />
  )
  let rightStyle = { paddingLeft: "4px", paddingRight: "4px" } as any
  if (isEnabled) {
    if (filterSet.source == FilterSource.build) {
      rightText = <span key="right-text">Build</span>
      rightStyle = null
    } else if (filterSet.source == FilterSource.runtime) {
      rightText = <span key="right-text">Runtime</span>
      rightStyle = null
    }
  }

  // isRadio indicates that clicking the button again won't turn it off,
  // behaving like a radio button.
  let leftClassName = "isRadio"
  let rightClassName = ""
  if (isEnabled) {
    leftClassName += " isEnabled"
    rightClassName += " isEnabled"
  }

  let history = useHistory()
  let l = useLocation()
  let onClick = () => {
    const search = createLogSearch(l.search, {
      level,
      source: FilterSource.all,
    })
    history.push({
      pathname: l.pathname,
      search: search.toString(),
    })
  }

  let [sourceMenuAnchor, setSourceMenuAnchor] = useState(null)
  let onMenuOpen = (e: any) => {
    setSourceMenuAnchor(e.currentTarget)
  }
  let sourceMenuOpen = !!sourceMenuAnchor

  return (
    <ButtonPill className={props.className}>
      <ButtonLeftPill
        className={leftClassName}
        onClick={onClick}
        analyticsName="ui.web.filterLevel"
        analyticsTags={{ level: level, source: props.filterSet.source }}
      >
        {leftText}
      </ButtonLeftPill>
      <ButtonRightPill
        style={rightStyle}
        className={rightClassName}
        onClick={onMenuOpen}
        analyticsName="ui.web.filterSourceMenu"
      >
        {rightText}
      </ButtonRightPill>
      <FilterSourceMenu
        id={`filterSource-${level}`}
        open={sourceMenuOpen}
        anchorEl={sourceMenuAnchor}
        filterSet={filterSet}
        level={level}
        alerts={alerts}
        onClose={() => setSourceMenuAnchor(null)}
      />
    </ButtonPill>
  )
}

export const FILTER_INPUT_DEBOUNCE = 500 // in ms
export const FILTER_FIELD_ID = "FilterTermTextInput"
export const FILTER_FIELD_TOOLTIP_ID = "FilterTermInfoTooltip"

function FilterTermFieldError({ error }: { error: string }) {
  return (
    <FieldErrorTooltip>
      <AlertIcon width="20" height="20" role="presentation" />
      {error}
    </FieldErrorTooltip>
  )
}

const filterTermTooltipContent = (
  <>
    RegExp should be wrapped in forward slashes, is case-insensitive, and is{" "}
    <a
      href="https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions"
      target="_blank"
    >
      parsed in JavaScript
    </a>
    .
  </>
)

const debounceFilterLogs = debounce((history: History, search: string) => {
  // Navigate to filtered logs with search query
  history.push({ search })
}, FILTER_INPUT_DEBOUNCE)

export function FilterTermField({ termFromUrl }: { termFromUrl: FilterTerm }) {
  const { input: initialTerm, state } = termFromUrl
  const location = useLocation()
  const history = useHistory()

  const [filterTerm, setFilterTerm] = useState(initialTerm ?? EMPTY_TERM)

  // If the location changes, reset the value of the input field based on url
  useEffect(() => {
    setFilterTerm(initialTerm)
  }, [location.pathname])

  /**
   * Note about term updates:
   * Debouncing allows us to wait to execute log filtration until a set
   * amount of time has passed without the filter term changing. To implement
   * debouncing, it's necessary to separate the term field's value from the url
   * search params, otherwise the field that a user types in doesn't update.
   * The term field updates without any debouncing, while the url search params
   * (which actually triggers log filtering) updates with the debounce delay.
   */
  const setTerm = (term: string, withDebounceDelay = true) => {
    setFilterTerm(term)

    const search = createLogSearch(location.search, { term })

    if (withDebounceDelay) {
      debounceFilterLogs(history, search.toString())
    } else {
      history.push({ search: search.toString() })
    }
  }

  const onChange = (
    event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => {
    const term = event.target.value ?? EMPTY_TERM
    setTerm(term)
  }

  const inputProps: InputProps = {
    startAdornment: (
      <InputAdornment position="start" disablePointerEvents={true}>
        <FilterSvg fill={Color.gray20} role="presentation" />
      </InputAdornment>
    ),
  }

  // If there's a search term, add a button to clear that term
  if (filterTerm) {
    const endAdornment = (
      <InputAdornment position="end">
        <ClearFilterTermTextButton
          analyticsName="ui.web.clearFilterTerm"
          onClick={() => setTerm(EMPTY_TERM, false)}
        >
          <SrOnly>Clear filter term</SrOnly>
          <CloseSvg fill={Color.grayLightest} role="presentation" />
        </ClearFilterTermTextButton>
      </InputAdornment>
    )

    inputProps.endAdornment = endAdornment
  }

  return (
    <>
      <FilterTermTextField
        aria-describedby={FILTER_FIELD_TOOLTIP_ID}
        error={state === TermState.Error}
        id={FILTER_FIELD_ID}
        helperText={
          isErrorTerm(termFromUrl) ? (
            <FilterTermFieldError error={termFromUrl.error} />
          ) : (
            ""
          )
        }
        InputProps={inputProps}
        onChange={onChange}
        placeholder="Filter by text or /regexp/"
        value={filterTerm}
        variant="outlined"
        analyticsName="ui.web.filterTerm"
      />
      <SrOnly component="label" htmlFor={FILTER_FIELD_ID}>
        Filter resource logs by text or /regexp/
      </SrOnly>
      <TiltInfoTooltip
        id={FILTER_FIELD_TOOLTIP_ID}
        dismissId="log-filter-term"
        title={filterTermTooltipContent}
        placement="right-end"
      />
    </>
  )
}

type CopyButtonProps = {
  podId: string
}

async function copyTextToClipboard(text: string, cb: () => void) {
  await navigator.clipboard.writeText(text)
  cb()
}

let TruncateText = styled.div`
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 250px;
`

export function CopyButton(props: CopyButtonProps) {
  let [showCopySuccess, setShowCopySuccess] = useState(false)

  let copyClick = () => {
    copyTextToClipboard(props.podId, () => {
      setShowCopySuccess(true)

      setTimeout(() => {
        setShowCopySuccess(false)
      }, 5000)
    })
  }

  let icon = showCopySuccess ? (
    <CheckmarkSvg width="20" height="20" />
  ) : (
    <CopySvg width="20" height="20" />
  )

  return (
    <ButtonRoot onClick={copyClick} analyticsName="ui.web.actionBar.copyPodID">
      {icon}
      <TruncateText style={{ marginLeft: "8px" }}>
        {props.podId} Pod ID
      </TruncateText>
    </ButtonRoot>
  )
}

let ActionBarRoot = styled.div`
  background-color: ${Color.gray10};
`

export let ActionBarTopRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid ${Color.gray40};
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
`

export let ActionBarBottomRow = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  border-bottom: 1px solid ${Color.gray40};
  min-height: ${SizeUnit(1)};
  padding-left: ${SizeUnit(0.5)};
  padding-right: ${SizeUnit(0.5)};
  padding-top: ${SizeUnit(0.35)};
  padding-bottom: ${SizeUnit(0.35)};
`

type ActionBarProps = {
  endpoints: Link[]
  podId: string
}

let EndpointSet = styled.div`
  display: flex;
  align-items: center;
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
`

export let Endpoint = styled.a`
  color: ${Color.gray70};
  transition: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
  }
`

let EndpointIcon = styled(LinkSvg)`
  fill: ${Color.gray70};
  margin-right: ${SizeUnit(0.25)};
`

// TODO(nick): Put this in a global React Context object with
// other page-level stuffs
function openEndpointUrl(url: string) {
  // We deliberately don't use rel=noopener. These are trusted tabs, and we want
  // to have a persistent link to them (so that clicking on the same link opens
  // the same tab).
  window.open(url, url)
}

export function OverviewWidgets(props: { buttons?: UIButton[] }) {
  if (!props.buttons?.length) {
    return null
  }

  return (
    <WidgetRoot key="widgets">
      {props.buttons?.map((b) => (
        <CustomActionButton uiButton={b} key={b.metadata?.name} />
      ))}
    </WidgetRoot>
  )
}

function DisableButtonSection(props: { button?: UIButton }) {
  if (!props.button) {
    return null
  }

  return <DisableButton uiButton={props.button} />
}

export default function OverviewActionBar(props: OverviewActionBarProps) {
  let { resource, filterSet, alerts, buttons } = props
  const logStore = useLogStore()
  const isSnapshot = usePathBuilder().isSnapshot()
  const isDisabled = resourceIsDisabled(resource)

  let endpoints = resource?.status?.endpointLinks || []
  let podId = resource?.status?.k8sResourceInfo?.podName || ""
  const resourceName = resource
    ? resource.metadata?.name || ""
    : ResourceName.all

  let endpointEls: JSX.Element[] = []
  if (endpoints.length && !isDisabled) {
    endpoints.forEach((ep, i) => {
      if (i !== 0) {
        endpointEls.push(<span key={`spacer-${i}`}>,&nbsp;</span>)
      }
      endpointEls.push(
        <Endpoint
          onClick={() =>
            void incr("ui.web.endpoint", { action: AnalyticsAction.Click })
          }
          href={ep.url}
          // We use ep.url as the target, so that clicking the link re-uses the tab.
          target={ep.url}
          key={ep.url}
        >
          <TruncateText>{ep.name || displayURL(ep)}</TruncateText>
        </Endpoint>
      )
    })
  }

  let topRowEls = new Array<JSX.Element>()
  if (endpointEls.length) {
    topRowEls.push(
      <EndpointSet key="endpointSet">
        <EndpointIcon />
        {endpointEls}
      </EndpointSet>
    )
  }
  if (podId && !isDisabled) {
    topRowEls.push(<CopyButton podId={podId} key="copyPodId" />)
  }

  const widgets = OverviewWidgets({ buttons: buttons?.default })
  if (widgets && !isDisabled) {
    topRowEls.push(widgets)
  }

  const topRow = topRowEls.length ? (
    <ActionBarTopRow key="top">{topRowEls}</ActionBarTopRow>
  ) : null

  // By default, add the disable toggle button regardless of a resource's disabled status
  const bottomRow: JSX.Element[] = [
    <DisableButtonSection
      key="toggleDisable"
      button={buttons?.toggleDisable}
    />,
  ]
  const disableButtonVisible = !!buttons?.toggleDisable
  const firstFilterButtonClass = disableButtonVisible ? "isCentered" : ""

  // Only display log filter controls if a resource is enabled
  if (!isDisabled) {
    bottomRow.push(
      <FilterRadioButton
        key="filterLevelAll"
        className={firstFilterButtonClass}
        level={FilterLevel.all}
        filterSet={filterSet}
        alerts={alerts}
      />
    )
    bottomRow.push(
      <FilterRadioButton
        key="filterLevelError"
        level={FilterLevel.error}
        filterSet={filterSet}
        alerts={alerts}
      />
    )
    bottomRow.push(
      <FilterRadioButton
        key="filterLevelWarn"
        level={FilterLevel.warn}
        filterSet={filterSet}
        alerts={alerts}
      />
    )
    bottomRow.push(
      <FilterTermField key="filterTermField" termFromUrl={filterSet.term} />
    )
    bottomRow.push(
      <LogActions
        key="logActions"
        resourceName={resourceName}
        isSnapshot={isSnapshot}
      />
    )
  }

  return (
    <ActionBarRoot>
      <OverviewActionBarKeyboardShortcuts
        logStore={logStore}
        resourceName={resourceName}
        endpoints={endpoints}
        openEndpointUrl={openEndpointUrl}
      />
      {topRow}
      <ActionBarBottomRow>{bottomRow}</ActionBarBottomRow>
    </ActionBarRoot>
  )
}
