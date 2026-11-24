import Menu from "@material-ui/core/Menu"
import MenuItem from "@material-ui/core/MenuItem"
import MoreHorizIcon from "@material-ui/icons/MoreHoriz"
import React, { useCallback, useEffect, useRef, useState } from "react"
import TimeAgo from "react-timeago"
import styled from "styled-components"
import { ButtonSet, updateButtonStatus } from "./ApiButton"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { usePersistentState } from "./BrowserStorage"
import { useHudErrorContext } from "./HudErrorContext"
import { InstrumentedButton } from "./instrumentedComponents"
import { displayURL, resolveURL } from "./links"
import { useLogAlertIndex } from "./LogStore"
import {
  CopyButton,
  DisableButtonSection,
  OverviewWidgets,
} from "./OverviewActionBar"
import { uiResourceToCell } from "./OverviewTable"
import { OverviewTableBuildButton } from "./OverviewTableBuildButton"
import OverviewTableStarResourceButton from "./OverviewTableStarResourceButton"
import OverviewTableStatus from "./OverviewTableStatus"
import { toggleTriggerMode } from "./OverviewTableTriggerModeToggle"
import { useStarredResources } from "./StarredResourcesContext"
import {
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
  ColorRGB,
} from "./style-helpers"
import { timeAgoFormatter } from "./timeFormatters"
import { startBuild } from "./trigger"
import {
  ResourceName,
  ResourceStatus,
  TriggerMode,
  UIButton,
  UIResource,
} from "./types"

type OverviewResourceHeaderProps = {
  // Shown even when the resource doesn't exist (e.g., a stale link).
  name: string
  resource?: UIResource
  buttons?: ButtonSet
  allButtons?: UIButton[]
  onClose: () => void
}

const HeaderRoot = styled.header`
  padding: ${SizeUnit(0.35)} ${SizeUnit(0.5)};
  background-color: ${ColorRGBA(ColorRGB.blue, ColorAlpha.almostTransparent)};
  border-bottom: 1px solid ${Color.gray40};
  font-family: ${Font.monospace};
  color: ${Color.gray70};
`

// Line 1: name, status, and the resource's main actions.
// When space is tight, the status wraps under the name; the actions stay put.
const TitleRow = styled.div`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.5)};
`

const TitleAndStatus = styled.div`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  flex: 1 1 auto;
  min-width: 0;
  gap: ${SizeUnit(0.15)} ${SizeUnit(0.5)};
`

const Title = styled.h2`
  margin: 0;
  font-size: ${FontSize.default};
  font-weight: normal;
  color: ${Color.offWhite};
  overflow-wrap: anywhere;

  &.has-error {
    color: ${Color.red};
  }
`

const StatusGroup = styled.div`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  min-width: 0;
  gap: ${SizeUnit(0.1)} ${SizeUnit(0.5)};

  /* The table stacks these two statuses; here they sit side by side */
  & > a {
    width: auto;
  }
  && > a + a {
    margin-top: 0;
  }
`

const Actions = styled.div`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.25)};
  flex-shrink: 0;
  margin-left: auto;
`

const CloseButton = styled(InstrumentedButton)`
  min-width: 0;
  padding: ${SizeUnit(0.25)};

  svg {
    fill: ${Color.gray60};
    width: 16px;
    height: 16px;
  }

  &:hover svg {
    fill: ${Color.offWhite};
  }
`

// The star sits right before the name. Unlike the table, it's always
// visible here, so you can see whether this resource is starred.
const StarSlot = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 0;

  svg {
    width: 15px;
    height: 15px;
  }
  && svg.is-unstarred {
    opacity: 1;
  }
  button:hover svg.is-unstarred {
    fill: ${Color.offWhite};
  }
`

const UpdatedAgo = styled.span`
  font-size: ${FontSize.small};
  color: ${Color.gray60};
  white-space: nowrap;
`

const MoreButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
  min-width: 0;
  padding: ${SizeUnit(0.15)};
  color: ${Color.gray60};
  border-radius: 4px;

  svg {
    width: 20px;
    height: 20px;
  }

  &:hover,
  &.isOpen {
    color: ${Color.offWhite};
    background-color: ${Color.gray30};
  }
`

const MenuHint = styled.div`
  font-size: ${FontSize.smallester};
  color: ${Color.gray50};
  margin-top: 2px;
`

const MenuLabel = styled.div`
  display: flex;
  flex-direction: column;

  &.isDanger {
    color: ${Color.red};
  }
`

const ShowMoreButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  min-width: 0;
  margin-left: auto;
  padding: ${SizeUnit(0.1)} ${SizeUnit(0.2)};
  color: ${Color.gray60};
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
  white-space: nowrap;

  &:hover {
    color: ${Color.offWhite};
    text-decoration: underline;
  }
`

// Shown under "Show more": less-used facts about the resource.
const MoreDetails = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: ${SizeUnit(0.25)} ${SizeUnit(1)};
  margin-top: ${SizeUnit(0.2)};
  font-size: ${FontSize.smallest};
  color: ${Color.offWhite};
`

const DetailLabel = styled.span`
  font-size: ${FontSize.smallester};
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: ${Color.gray60};
  margin-right: ${SizeUnit(0.25)};
`

const DetailItem = styled.div`
  display: flex;
  align-items: center;
  min-width: 0;
`

// Line 2: where the resource is running. The first URL is the one most
// people want, so it's the biggest thing after the name.
const UrlRow = styled.div`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: ${SizeUnit(0.15)} ${SizeUnit(0.5)};
  margin-top: ${SizeUnit(0.2)};
`

const PrimaryUrl = styled.div`
  display: flex;
  align-items: center;
  min-width: 0;
  max-width: 100%;
`

const Endpoint = styled.a`
  display: flex;
  align-items: center;
  min-width: 0;
  color: ${Color.blueLight};
  font-size: ${FontSize.smallest};
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }

  svg {
    fill: currentColor;
    flex-shrink: 0;
    margin-right: ${SizeUnit(0.15)};
  }

  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &.isPrimary {
    font-size: ${FontSize.default};
  }
`

const EndpointName = styled.small`
  margin-left: ${SizeUnit(0.25)};
  color: ${Color.gray60};
  font-size: ${FontSize.smallest};
  flex-shrink: 0;
`

const CopyUrlButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
  flex-shrink: 0;
  min-width: 0;
  padding: ${SizeUnit(0.15)};
  margin-left: ${SizeUnit(0.1)};
  color: ${Color.gray60};

  svg {
    fill: currentColor;
    width: 16px;
    height: 16px;
  }

  &:hover {
    color: ${Color.offWhite};
  }

  &.isCopied {
    color: ${Color.green};
  }
`

function CopyUrl(props: { url: string }) {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) {
      return
    }
    const timeout = setTimeout(() => setCopied(false), 2000)
    return () => clearTimeout(timeout)
  }, [copied])
  const onClick = async () => {
    await navigator.clipboard.writeText(props.url)
    setCopied(true)
  }
  const label = copied ? "Copied!" : "Copy URL"
  return (
    <CopyUrlButton
      onClick={onClick}
      className={copied ? "isCopied" : ""}
      aria-label={label}
      title={label}
    >
      {copied ? (
        <CheckmarkSvg role="presentation" />
      ) : (
        <CopySvg role="presentation" />
      )}
    </CopyUrlButton>
  )
}

function EndpointLink(props: {
  url: string
  name?: string
  isPrimary?: boolean
}) {
  const url = resolveURL(props.url)
  // The main URL shows the address itself; its name (if any) is a label.
  const text = props.isPrimary ? displayURL(url) : props.name || displayURL(url)
  // We use the url as the target, so that clicking the link re-uses the tab.
  return (
    <Endpoint
      href={url}
      target={url}
      title={`Open ${url}`}
      className={props.isPrimary ? "isPrimary" : ""}
    >
      <LinkSvg role="presentation" />
      <span>{text}</span>
      {props.isPrimary && props.name && (
        <EndpointName>{props.name}</EndpointName>
      )}
    </Endpoint>
  )
}

function isManualMode(mode: TriggerMode): boolean {
  return (
    mode === TriggerMode.TriggerModeManualWithAutoInit ||
    mode === TriggerMode.TriggerModeManual
  )
}

// Settings you change rarely (trigger mode, disabling), tucked away so the
// header stays focused on the resource itself.
function ResourceMoreMenu(props: {
  name: string
  triggerMode?: TriggerMode
  disableButton?: UIButton
}) {
  const { name, triggerMode, disableButton } = props
  const anchorRef = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [confirmingDisable, setConfirmingDisable] = useState(false)
  const { setError } = useHudErrorContext()

  const close = () => {
    setOpen(false)
    setConfirmingDisable(false)
  }

  const items: JSX.Element[] = []
  if (triggerMode !== undefined) {
    const isManual = isManualMode(triggerMode)
    items.push(
      <MenuItem
        key="mode"
        onClick={() => {
          toggleTriggerMode(
            name,
            isManual
              ? TriggerMode.TriggerModeAuto
              : TriggerMode.TriggerModeManual
          )
          close()
        }}
      >
        <MenuLabel>
          {isManual ? "Switch to auto updates" : "Switch to manual updates"}
          <MenuHint>
            {isManual
              ? "Now manual: file changes don’t trigger updates"
              : "Now auto: file changes trigger updates"}
          </MenuHint>
        </MenuLabel>
      </MenuItem>
    )
  }
  if (disableButton) {
    // Disabling stops the resource, so it takes a second click to confirm.
    items.push(
      <MenuItem
        key="disable"
        onClick={async () => {
          if (!confirmingDisable) {
            setConfirmingDisable(true)
            return
          }
          close()
          try {
            await updateButtonStatus(disableButton, {})
          } catch (err) {
            setError(`Error disabling resource: ${err}`)
          }
        }}
      >
        <MenuLabel className={confirmingDisable ? "isDanger" : ""}>
          {confirmingDisable ? "Click again to disable" : "Disable resource"}
          <MenuHint>Stops it until you enable it again</MenuHint>
        </MenuLabel>
      </MenuItem>
    )
  }

  if (items.length === 0) {
    return null
  }

  return (
    <>
      <MoreButton
        ref={anchorRef}
        onClick={() => setOpen(true)}
        className={open ? "isOpen" : ""}
        aria-label="More actions"
        title="More actions"
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <MoreHorizIcon role="presentation" />
      </MoreButton>
      <Menu
        anchorEl={anchorRef.current}
        open={open}
        onClose={close}
        disableScrollLock={true}
        getContentAnchorEl={null}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
      >
        {items}
      </Menu>
    </>
  )
}

export default function OverviewResourceHeader(
  props: OverviewResourceHeaderProps
) {
  const { name, resource, buttons, allButtons, onClose } = props
  const alertIndex = useLogAlertIndex()
  const starCtx = useStarredResources()
  const onStartBuild = useCallback(() => startBuild(name), [name])
  const [showMore, setShowMore] = usePersistentState(
    "resource-header-show-more",
    false
  )

  const close = (
    <CloseButton
      onClick={onClose}
      aria-label="Close details (Esc)"
      title="Close details (Esc)"
    >
      <CloseSvg role="presentation" />
    </CloseButton>
  )

  // The combined logs, or a stale link to a resource that doesn't exist:
  // just the title and close.
  if (!resource) {
    let title = name
    if (name === ResourceName.all) {
      title = "All Resources"
    } else if (name === ResourceName.starred) {
      title = "Starred Resources"
    }
    return (
      <HeaderRoot aria-label={`${title} details`}>
        <TitleRow>
          <Title>{title}</Title>
          <Actions>{close}</Actions>
        </TitleRow>
      </HeaderRoot>
    )
  }

  // Reuse the table's view of this resource, so the header and the
  // table row always agree.
  const cell = uiResourceToCell(resource, allButtons, alertIndex)
  const status = cell.statusLine
  const isDisabled = status.runtimeStatus === ResourceStatus.Disabled
  const isTiltfile = name === ResourceName.tiltfile
  const hasError =
    status.buildStatus === ResourceStatus.Unhealthy ||
    status.runtimeStatus === ResourceStatus.Unhealthy

  const endpoints = isDisabled ? [] : cell.endpoints.filter((ep) => !!ep.url)
  const [primary, ...others] = endpoints
  const widgets = !isDisabled && buttons?.default.length ? buttons.default : []
  const podId = isDisabled ? "" : cell.podId
  const hasSecondRow = !!primary || widgets.length > 0 || !!podId

  return (
    <HeaderRoot aria-label={`${name} details`}>
      <TitleRow>
        <TitleAndStatus>
          <StarSlot>
            <OverviewTableStarResourceButton
              resourceName={name}
              ctx={starCtx}
            />
          </StarSlot>
          <Title className={hasError ? "has-error" : ""}>{name}</Title>
          <StatusGroup>
            {!isDisabled && (
              <OverviewTableStatus
                status={status.buildStatus}
                lastBuildDur={status.lastBuildDur}
                isBuild={true}
                resourceName={name}
                hold={status.hold}
              />
            )}
            <OverviewTableStatus
              status={status.runtimeStatus}
              resourceName={name}
            />
            {!isDisabled && cell.lastDeployTime && (
              <UpdatedAgo
                aria-label="Last updated"
                title={`Last updated ${new Date(
                  cell.lastDeployTime
                ).toLocaleString()}`}
              >
                <TimeAgo
                  date={cell.lastDeployTime}
                  formatter={timeAgoFormatter}
                />
              </UpdatedAgo>
            )}
          </StatusGroup>
        </TitleAndStatus>
        <Actions>
          {!isDisabled && (
            <OverviewTableBuildButton
              hasPendingChanges={cell.trigger.hasPendingChanges}
              hasBuilt={cell.trigger.hasBuilt}
              isBuilding={cell.trigger.isBuilding}
              triggerMode={cell.mode}
              isQueued={cell.trigger.isQueued}
              onStartBuild={onStartBuild}
              stopBuildButton={cell.buttons.stopBuild}
              shortcut="r"
            />
          )}
          {/* When disabled, enabling is the main thing to do, so it stays out
              in the open. Otherwise, disabling lives in the "⋯" menu. */}
          {isDisabled && (
            <DisableButtonSection button={buttons?.toggleDisable} />
          )}
          <ResourceMoreMenu
            name={name}
            triggerMode={isDisabled || isTiltfile ? undefined : cell.mode}
            disableButton={isDisabled ? undefined : buttons?.toggleDisable}
          />
          {close}
        </Actions>
      </TitleRow>
      {hasSecondRow && (
        <UrlRow aria-label="URLs">
          {primary && (
            <PrimaryUrl>
              <EndpointLink
                url={primary.url!}
                name={primary.name}
                isPrimary={true}
              />
              <CopyUrl url={resolveURL(primary.url!)} />
            </PrimaryUrl>
          )}
          {others.map((ep) => (
            <EndpointLink key={ep.url} url={ep.url!} name={ep.name} />
          ))}
          {widgets.length > 0 && <OverviewWidgets buttons={widgets} />}
          {podId && (
            <ShowMoreButton
              onClick={() => setShowMore(!showMore)}
              aria-expanded={showMore}
            >
              {showMore ? "Show less" : "Show more"}
            </ShowMoreButton>
          )}
        </UrlRow>
      )}
      {podId && showMore && (
        <MoreDetails>
          <DetailItem>
            <DetailLabel>Pod ID</DetailLabel>
            <CopyButton podId={podId} />
          </DetailItem>
        </MoreDetails>
      )}
    </HeaderRoot>
  )
}
