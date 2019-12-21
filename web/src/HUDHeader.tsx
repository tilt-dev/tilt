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
  padding: ${s.SizeUnit(0.5)};
  background-color: ${s.Color.grayDarkest};
`

let Title = styled.h2`
  font-family: ${s.Font.sansSerif};
  font-size: ${s.FontSize.default};
  margin: 0;
`

let PodStatus = styled.span``

let PortForward = styled.span``

let PodId = styled.span``

let SnapshotButton = styled.button`
  border: 0;
  background-color: transparent;
  color: ${s.Color.blue};
`

class HUDHeader extends PureComponent<HUDHeaderProps> {
  render() {
    let name = this.props.name ?? "Overview"
    let podID = this.props.podID
    let podStatus = this.props.podStatus
    let podIDEl = podID && (
      <>
        {podID && (
          <div className="resourceInfo">
            <div className="resourceInfo-value">
              {podID} ({podStatus})
            </div>
          </div>
        )}
      </>
    )

    let endpoints = this.props.endpoints ?? []
    let endpointsEl = endpoints?.length > 0 && (
      <div className="resourceInfo">
        <div className="resourceInfo-label">
          Port Forward{endpoints?.length > 1 ? "s" : ""}:
        </div>
        {endpoints?.map(ep => (
          <a
            className="resourceInfo-value"
            href={ep}
            target="_blank"
            rel="noopener noreferrer"
            key={ep}
          >
            {ep}
          </a>
        ))}
      </div>
    )

    return (
      <Root>
        <Title>{name}</Title>

        <PodStatus>({podStatus})</PodStatus>
        <PodId>{podID}</PodId>
        <PortForward>{endpointsEl}</PortForward>

        {this.renderSnapshotButton()}
      </Root>
    )
  }

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
          <SnapshotSvg className="SecondaryNav-snapshotSvg" />
          <span>
            Create a <br />
            Snapshot
          </span>
        </SnapshotButton>
      )
  }
}

export default HUDHeader
