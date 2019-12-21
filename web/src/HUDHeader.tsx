import React, { PureComponent } from "react"
import styled from "styled-components"
import * as s from "./style-helpers"
import { SnapshotHighlight } from "./types"
import { ReactComponent as SnapshotSvg } from "./assets/svg/snapshot.svg"

type HUDHeaderProps = {
  name?: string
  podID?: string
  endpoints?: string[]
  podStatus?: string
  showSnapshotButton?: boolean
  handleOpenModal?: () => void
  highlight?: SnapshotHighlight | null
}

let Root = styled.div`
  display: flex;
  padding: ${s.SizeUnit(0.5)};
  background-color: ${s.Color.grayDarkest};
`

let PodStatus = styled.span``

let PortForward = styled.span``

let PortForwardLabel = styled.span``

let PortForwardLink = styled.a``

let PodId = styled.span``

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

class HUDHeader extends PureComponent<HUDHeaderProps> {
  renderSnapshotButton() {
    let highlight = this.props.highlight

    if (this.props.showSnapshotButton)
      return (
        <SnapshotButton
          onClick={this.props.handleOpenModal}
          className={`snapshotButton ${
            highlight ? "isHighlighted" : ""
          }`}
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
      <PortForward>
        <PortForwardLabel>
          Port-Forward{endpoints?.length > 1 ? "s" : ""}:
        </PortForwardLabel>
        {endpoints?.map(ep => (
          <PortForwardLink
            className="resourceInfo-value"
            href={ep}
            target="_blank"
            rel="noopener noreferrer"
            key={ep}
          >
            {ep}
          </PortForwardLink>
        ))}
      </PortForward>
    )

    return (
      <Root>
        <PodStatus>({podStatus})</PodStatus>
        <PodId>{podID}</PodId>
        {endpointsEl}
        {this.renderSnapshotButton()}
      </Root>
    )
  }
}

export default HUDHeader
