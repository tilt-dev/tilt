import Menu from "@material-ui/core/Menu"
import MenuItem from "@material-ui/core/MenuItem"
import { PopoverOrigin } from "@material-ui/core/Popover"
import { makeStyles } from "@material-ui/core/styles"
import ExpandMoreIcon from "@material-ui/icons/ExpandMore"
import moment from "moment"
import React, { useRef, useState } from "react"
import { useHistory } from "react-router"
import styled from "styled-components"
import { Alert } from "./alerts"
import { incr } from "./analytics"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { displayURL } from "./links"
import LogActions from "./LogActions"
import { FilterLevel, FilterSet, FilterSource } from "./logfilters"
import { useLogStore } from "./LogStore"
import OverviewActionBarKeyboardShortcuts from "./OverviewActionBarKeyboardShortcuts"
import { usePathBuilder } from "./PathBuilder"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"
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
  buttons?: UIButton[]
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
  let { id, anchorEl, level, open, filterSet, onClose } = props
  let alerts = props.alerts || []

  let classes = useMenuStyles()
  let history = useHistory()
  let l = history.location
  let onClick = (e: any) => {
    let source = e.currentTarget.getAttribute("data-filter")
    let search = new URLSearchParams(l.search)
    search.set("source", source)
    search.set("level", level)
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

let ButtonRoot = styled(InstrumentedButton)`
  font-family: ${Font.sansSerif};
  display: flex;
  align-items: center;
  padding: 8px 12px;
  margin: 0;

  background: ${Color.grayDark};

  border: 1px solid ${Color.grayLighter};
  box-sizing: border-box;
  border-radius: 4px;
  cursor: pointer;
  transition: color ${AnimDuration.default} ease,
    border-color ${AnimDuration.default} ease;
  color: ${Color.gray7};

  &.isEnabled {
    background: ${Color.gray7};
    color: ${Color.grayDark};
    border-color: ${Color.grayDarker};
  }
  &.isEnabled.isRadio {
    pointer-events: none;
  }

  & .fillStd {
    fill: ${Color.gray7};
    transition: fill ${AnimDuration.default} ease;
  }
  &.isEnabled .fillStd {
    fill: ${Color.grayDark};
  }

  &:active,
  &:focus {
    outline: none;
    border-color: ${Color.grayLightest};
  }
  &.isEnabled:active,
  &.isEnabled:focus {
    outline: none;
    border-color: ${Color.grayDarkest};
  }

  &:hover {
    color: ${Color.blue};
    border-color: ${Color.blue};
  }
  &:hover .fillStd {
    fill: ${Color.blue};
  }
  &.isEnabled:hover {
    color: ${Color.blueDark};
    border-color: ${Color.blueDark};
  }
  &.isEnabled:hover .fillStd {
    fill: ${Color.blue};
  }
`

const WidgetRoot = styled.div`
  display: flex;
  ${ButtonRoot} + ${ButtonRoot} {
    margin-left: ${SizeUnit(0.125)};
  }
`

let ButtonPill = styled.div`
  display: flex;
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

type FilterRadioButtonProps = {
  // The level that this button toggles.
  level: FilterLevel

  // The current filter set.
  filterSet: FilterSet

  // All the alerts for the current resource.
  alerts?: Alert[]
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
  let l = history.location
  let onClick = () => {
    let search = new URLSearchParams(l.search)
    search.set("level", level)
    search.set("source", "")
    history.push({
      pathname: l.pathname,
      search: search.toString(),
    })
  }

  let rightPillRef = useRef(null as any)
  let [sourceMenuAnchor, setSourceMenuAnchor] = useState(null)
  let onMenuOpen = (e: any) => {
    setSourceMenuAnchor(e.currentTarget)
  }
  let sourceMenuOpen = !!sourceMenuAnchor

  return (
    <ButtonPill style={{ marginRight: SizeUnit(0.5) }}>
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

function CopyButton(props: CopyButtonProps) {
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
  background-color: ${Color.grayDarkest};
`

export let ActionBarTopRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid ${Color.grayLighter};
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
`

let ActionBarBottomRow = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  border-bottom: 1px solid ${Color.grayLighter};
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
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
  color: ${Color.gray7};
  transition: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
  }
`

let EndpointIcon = styled(LinkSvg)`
  fill: ${Color.gray7};
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

function ApiButton(props: { button: UIButton }) {
  const [loading, setLoading] = useState(false)
  const onClick = async () => {
    const toUpdate = {
      metadata: { ...props.button.metadata },
      status: { ...props.button.status },
    } as UIButton
    // apiserver's date format time is _extremely_ strict to the point that it requires the full
    // six-decimal place microsecond precision, e.g. .000Z will be rejected, it must be .000000Z
    // so use an explicit RFC3339 moment format to ensure it passes
    toUpdate.status!.lastClickedAt = moment().format(
      "YYYY-MM-DDTHH:mm:ss.SSSSSSZ"
    )

    // TODO(milas): currently the loading state just disables the button for the duration of
    //  the AJAX request to avoid duplicate clicks - there is no progress tracking at the
    //  moment, so there's no fancy spinner animation or propagation of result of action(s)
    //  that occur as a result of click right now
    setLoading(true)
    const url = `/proxy/apis/tilt.dev/v1alpha1/uibuttons/${
      toUpdate.metadata!.name
    }/status`
    try {
      await fetch(url, {
        method: "PUT",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(toUpdate),
      })
    } finally {
      setLoading(false)
    }
  }
  // button text is not included in analytics name since that can be user data
  return (
    <ButtonRoot
      analyticsName={"ui.web.uibutton"}
      onClick={onClick}
      disabled={loading}
    >
      {props.button.spec?.text ?? "Button"}
    </ButtonRoot>
  )
}

export function OverviewWidgets(props: { buttons?: UIButton[] }) {
  if (!props.buttons?.length) {
    return null
  }

  return (
    <WidgetRoot key="widgets">
      {props.buttons?.map((b) => (
        <ApiButton button={b} key={b.metadata?.name} />
      ))}
    </WidgetRoot>
  )
}

export default function OverviewActionBar(props: OverviewActionBarProps) {
  let { resource, filterSet, alerts, buttons } = props
  let endpoints = resource?.status?.endpointLinks || []
  let podId = resource?.status?.k8sResourceInfo?.podName || ""
  const resourceName = resource
    ? resource.metadata?.name || ""
    : ResourceName.all
  const isSnapshot = usePathBuilder().isSnapshot()
  const logStore = useLogStore()

  let endpointEls: any = []
  endpoints.forEach((ep, i) => {
    if (i !== 0) {
      endpointEls.push(<span key={`spacer-${i}`}>,&nbsp;</span>)
    }
    endpointEls.push(
      <Endpoint
        onClick={() => void incr("ui.web.endpoint", { action: "click" })}
        href={ep.url}
        // We use ep.url as the target, so that clicking the link re-uses the tab.
        target={ep.url}
        key={ep.url}
      >
        <TruncateText>{ep.name || displayURL(ep)}</TruncateText>
      </Endpoint>
    )
  })

  let topRowEls = new Array<JSX.Element>()
  if (endpointEls.length) {
    topRowEls.push(
      <EndpointSet key="endpointSet">
        <EndpointIcon />
        {endpointEls}
      </EndpointSet>
    )
  }
  if (podId) {
    topRowEls.push(<CopyButton podId={podId} key="copyPodId" />)
  }

  const widgets = OverviewWidgets({ buttons })
  if (widgets) {
    topRowEls.push(widgets)
  }

  const topRow = topRowEls.length ? (
    <ActionBarTopRow key="top">{topRowEls}</ActionBarTopRow>
  ) : null

  return (
    <ActionBarRoot>
      <OverviewActionBarKeyboardShortcuts
        logStore={logStore}
        resourceName={resourceName}
        endpoints={endpoints}
        openEndpointUrl={openEndpointUrl}
      />
      {topRow}
      <ActionBarBottomRow>
        <FilterRadioButton
          level={FilterLevel.all}
          filterSet={props.filterSet}
          alerts={alerts}
        />
        <FilterRadioButton
          level={FilterLevel.error}
          filterSet={props.filterSet}
          alerts={alerts}
        />
        <FilterRadioButton
          level={FilterLevel.warn}
          filterSet={props.filterSet}
          alerts={alerts}
        />
        <LogActions resourceName={resourceName} isSnapshot={isSnapshot} />
      </ActionBarBottomRow>
    </ActionBarRoot>
  )
}
