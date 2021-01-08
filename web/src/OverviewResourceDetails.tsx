import React, { useState } from "react"
import styled from "styled-components"
import { incr } from "./analytics"
import { ReactComponent as CheckmarkSvg } from "./assets/svg/checkmark.svg"
import { ReactComponent as CopySvg } from "./assets/svg/copy.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { displayURL } from "./links"
import OverviewLogPane from "./OverviewLogPane"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"

type OverviewResourceDetailsProps = {
  resource?: Proto.webviewResource
}

let OverviewResourceDetailsRoot = styled.div`
  display: flex;
  flex-grow: 1;
  flex-direction: column;
`

let CopyButtonRoot = styled.button`
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

  & .fillStd {
    fill: ${Color.gray7};
    transition: fill ${AnimDuration.default} ease;
  }
  &:active,
  &:focus {
    outline: none;
    border-color: ${Color.grayLightest};
  }
  &:hover {
    color: ${Color.blue};
    border-color: ${Color.blue};
  }
  &:hover .fillStd {
    fill: ${Color.blue};
  }
`

type CopyButtonProps = {
  podId: string
}

async function copyTextToClipboard(text: string, cb: () => void) {
  await navigator.clipboard.writeText(text)
  cb()
}

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
    <CopyButtonRoot onClick={copyClick}>
      {icon}
      <div style={{ marginLeft: "8px" }}>Copy Pod ID</div>
    </CopyButtonRoot>
  )
}

let ActionBarRoot = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
  background-color: ${Color.grayDarkest};
  border-bottom: 1px solid ${Color.grayLighter};
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

function ActionBar(props: ActionBarProps) {
  let endpointEls: any = []
  props.endpoints.forEach((ep, i) => {
    if (i !== 0) {
      endpointEls.push(<span>,&nbsp;</span>)
    }
    endpointEls.push(
      <Endpoint
        onClick={() => void incr("ui.web.endpoint", { action: "click" })}
        href={ep.url}
        // We use ep.url as the target, so that clicking the link re-uses the tab.
        target={ep.url}
        key={ep.url}
      >
        {ep.name || displayURL(ep)}
      </Endpoint>
    )
  })

  let copyButton = props.podId ? (
    <CopyButton podId={props.podId} />
  ) : (
    <div>&nbsp;</div>
  )
  return (
    <ActionBarRoot>
      {endpointEls.length ? (
        <EndpointSet>
          <EndpointIcon />
          {endpointEls}
        </EndpointSet>
      ) : (
        <EndpointSet />
      )}
      {copyButton}
    </ActionBarRoot>
  )
}

export default function OverviewResourceDetails(
  props: OverviewResourceDetailsProps
) {
  let resource = props.resource
  let manifestName = resource?.name || ""
  let endpoints = resource?.endpointLinks || []
  let podId = resource?.podID || ""
  return (
    <OverviewResourceDetailsRoot>
      <ActionBar podId={podId} endpoints={endpoints} />
      <OverviewLogPane manifestName={manifestName} />
    </OverviewResourceDetailsRoot>
  )
}
