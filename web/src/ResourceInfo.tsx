import React, { PureComponent } from "react"
import styled from "styled-components"
import * as s from "./style-helpers"
import { SnapshotHighlight } from "./types"
import { ReactComponent as SnapshotSvg } from "./assets/svg/snapshot.svg"
import ResourceInfoKeyboardShortcuts from "./ResourceInfoKeyboardShortcuts"
import { incr } from "./analytics"

type Link = Proto.webviewLink

type HUDHeaderProps = {
  podID?: string
  endpoints?: Link[]
  podStatus?: string
  showSnapshotButton: boolean
  highlight: SnapshotHighlight | null

  // TODO(nick): This needs a better name
  handleOpenModal: () => void
}

let Root = styled.div`
  display: flex;
  align-items: center;
  height: ${s.Height.statusHeader}px;
  padding-left: ${s.SizeUnit(0.5)};
  padding-right: ${s.SizeUnit(0.25)};
`
let ResourceInfoStyle = styled.div`
  flex: 1;
  display: flex;
  padding-right: ${s.SizeUnit(0.5)};
`

let PodStatus = styled.span`
  font-family: ${s.Font.sansSerif};
  font-size: ${s.FontSize.small};
  flex: 1;
`

let PodId = styled.span``

let Endpoints = styled.span`
  ${PodId} + & {
    margin-left: ${s.SizeUnit(0.5)};
    border-left: 1px solid ${s.Color.gray};
    padding-left: ${s.SizeUnit(0.5)};
  }
`

let EndpointsLabel = styled.span`
  color: ${s.Color.grayLight};
  margin-right: ${s.SizeUnit(0.25)};

  ${s.mixinHideOnSmallScreen}
`

let Endpoint = styled.a`
  & + & {
    padding-left: ${s.SizeUnit(0.25)};
    border-left: 1px dotted ${s.Color.grayLight};
    margin-left: ${s.SizeUnit(0.25)};
  }
`

let SnapshotButton = styled.button`
  border: 1px solid transparent;
  font-family: ${s.Font.sansSerif};
  font-size: ${s.FontSize.smallest};
  background-color: transparent;
  color: ${s.Color.blue};
  display: flex;
  align-items: center;
  box-sizing: border-box;
  padding-left: ${s.SizeUnit(0.5)};
  padding-right: ${s.SizeUnit(0.5)};
  padding-top: ${s.SizeUnit(0.25)};
  padding-bottom: ${s.SizeUnit(0.25)};
  transition: border-color;
  transition-duration: ${s.AnimDuration.default};
  text-decoration: none;
  cursor: pointer;

  &:hover {
    background-color: ${s.Color.grayDark};
    border-color: ${s.Color.blue};
  }

  &.isHighlighted {
    border-color: ${s.Color.blue};
  }
`

let SnapshotButtonSvg = styled(SnapshotSvg)`
  margin-right: ${s.SizeUnit(0.25)};
`

// TODO(nick): Put this in a global React Context object with
// other page-level stuffs
function openEndpointUrl(url: string) {
  // We deliberately don't use rel=noopener. These are trusted tabs, and we want
  // to have a persistent link to them (so that clicking on the same link opens
  // the same tab).
  window.open(url, url)
}

class ResourceInfo extends PureComponent<HUDHeaderProps> {
  renderSnapshotButton() {
    let highlight = this.props.highlight

    if (this.props.showSnapshotButton)
      return (
        <SnapshotButton
          onClick={this.props.handleOpenModal}
          className={`snapshotButton ${highlight ? "isHighlighted" : ""}`}
        >
          <SnapshotButtonSvg />
          <span>
            Create a <br />
            Snapshot
          </span>
        </SnapshotButton>
      )
  }

  render() {
    let podStatus = this.props.podStatus
    let podID = this.props.podID

    let endpoints = this.props.endpoints ?? []
    let endpointsEl = endpoints?.length > 0 && (
      <Endpoints id="endpoints">
        <EndpointsLabel>
          Endpoint{endpoints?.length > 1 ? "s" : ""}:
        </EndpointsLabel>

        {endpoints?.map(ep => (
          <Endpoint
            onClick={() => void incr("ui.web.endpoint", { action: "click" })}
            href={ep.url}
            // We use ep.url as the target, so that clicking the link re-uses the tab.
            target={ep.url}
            key={ep.url}
          >
            {ep.name || displayURL(ep)}
          </Endpoint>
        ))}
      </Endpoints>
    )

    return (
      <Root>
        <ResourceInfoKeyboardShortcuts
          openEndpointUrl={openEndpointUrl}
          showSnapshotButton={this.props.showSnapshotButton}
          openSnapshotModal={this.props.handleOpenModal}
          endpoints={this.props.endpoints}
        />
        <ResourceInfoStyle>
          <PodStatus>{podStatus}</PodStatus>
          {podID && <PodId>{podID}</PodId>}
          {endpointsEl}
        </ResourceInfoStyle>
        {this.renderSnapshotButton()}
      </Root>
    )
  }
}

function displayURL(li: Link): string {
  let url = li.url?.replace(/^(http:\/\/)/, "")
  url = url?.replace(/^(https:\/\/)/, "")
  url = url?.replace(/^(www\.)/, "")
  return url || ""
}

export default ResourceInfo
