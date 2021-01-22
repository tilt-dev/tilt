import React, { useState } from "react"
import { useHistory } from "react-router"
import styled from "styled-components"
import { incr } from "./analytics"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { displayURL } from "./links"
import { FilterLevel, FilterSet } from "./logfilters"
import OverviewActionBarKeyboardShortcuts from "./OverviewActionBarKeyboardShortcuts"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"

type OverviewActionBarProps = {
  resource?: Proto.webviewResource
  filterSet: FilterSet
}

let ButtonRoot = styled.button`
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

type FilterRadioButtonProps = {
  // The level that this button toggles.
  level: FilterLevel

  // The current filter set.
  filterSet: FilterSet
}

function FilterRadioButton(props: FilterRadioButtonProps) {
  let level = props.level
  let text = "All Logs"
  if (level === FilterLevel.warn) {
    text = "Warnings"
  } else if (level === FilterLevel.error) {
    text = "Errors"
  }
  let isEnabled = level === props.filterSet.level

  // isRadio indicates that clicking the button again won't turn it off,
  // behaving like a radio button.
  let className = "isRadio"
  if (isEnabled) {
    className += " isEnabled"
  }

  let history = useHistory()
  let l = history.location
  let onClick = () => {
    let search = new URLSearchParams(l.search)
    search.set("level", level)
    history.push({
      pathname: l.pathname,
      search: search.toString(),
    })
  }
  return (
    <ButtonRoot
      style={{ marginRight: "16px" }}
      className={className}
      onClick={onClick}
    >
      {text}
    </ButtonRoot>
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
    <ButtonRoot onClick={copyClick}>
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

let ActionBarTopRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid ${Color.grayLighter};
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
`

let ActionBarBottomRow = styled.div`
  display: flex;
  align-items: center;
  border-bottom: 1px solid ${Color.grayLighter};
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
`

type ActionBarProps = {
  endpoints: Proto.webviewLink[]
  podId: string
}

let EndpointSet = styled.div`
  display: flex;
  align-items: center;
  font-family: ${Font.monospace};
  font-size: ${FontSize.small};
`

let Endpoint = styled.a`
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

export default function OverviewActionBar(props: OverviewActionBarProps) {
  let { resource, filterSet } = props
  let manifestName = resource?.name || ""
  let endpoints = resource?.endpointLinks || []
  let podId = resource?.podID || ""

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

  let copyButton = podId ? <CopyButton podId={podId} /> : <div>&nbsp;</div>

  let topRow =
    endpointEls.length || podId ? (
      <ActionBarTopRow key="top">
        {endpointEls.length ? (
          <EndpointSet>
            <EndpointIcon />
            {endpointEls}
          </EndpointSet>
        ) : (
          <EndpointSet />
        )}
        {copyButton}
        <OverviewActionBarKeyboardShortcuts
          endpoints={endpoints}
          openEndpointUrl={openEndpointUrl}
        />
      </ActionBarTopRow>
    ) : null

  return (
    <ActionBarRoot>
      {topRow}
      <ActionBarBottomRow>
        <FilterRadioButton
          level={FilterLevel.all}
          filterSet={props.filterSet}
        />
        <FilterRadioButton
          level={FilterLevel.warn}
          filterSet={props.filterSet}
        />
        <FilterRadioButton
          level={FilterLevel.error}
          filterSet={props.filterSet}
        />
      </ActionBarBottomRow>
    </ActionBarRoot>
  )
}
