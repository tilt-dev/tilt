import React, { PureComponent } from "react"
import { ResourceView, SnapshotHighlight } from "./types"
import { ReactComponent as SnapshotSvg } from "./assets/svg/snapshot.svg"
import "./TopBar.scss"
import TabNav from "./TabNav"

type TopBarProps = {
  logUrl: string
  alertsUrl: string
  resourceView: ResourceView
  numberOfAlerts: number
  showSnapshotButton: boolean
  snapshotOwner: string | null
  handleOpenModal: () => void
  highlight: SnapshotHighlight | null
  teamSnapshotsUrl: string | null
  teamUpdatesUrl: string | null
  facetsUrl: string | null
}

class TopBar extends PureComponent<TopBarProps> {
  render() {
    return (
      <div className="TopBar">
        <TabNav
          logUrl={this.props.logUrl}
          alertsUrl={this.props.alertsUrl}
          facetsUrl={this.props.facetsUrl}
          resourceView={this.props.resourceView}
          numberOfAlerts={this.props.numberOfAlerts}
        />
        <div className="TopBar-tools">
          {this.maybeRenderTeamSnapshotsButton()}
          {this.maybeRenderTeamUpdatesButton()}
          {this.props.showSnapshotButton
            ? this.renderSnapshotModal()
            : this.renderSnapshotOwner()}
        </div>
      </div>
    )
  }

  maybeRenderTeamSnapshotsButton() {
    if (!this.props.teamSnapshotsUrl || this.isSnapshot()) {
      return null
    }
    return (
      <a
        href={this.props.teamSnapshotsUrl}
        target="_blank"
        rel="noreferrer noopener"
        className={`TopBar-toolsButton`}
      >
        <span>Team Snapshots</span>
      </a>
    )
  }

  isSnapshot() {
    return !!this.props.snapshotOwner
  }

  maybeRenderTeamUpdatesButton() {
    if (!this.props.teamUpdatesUrl || this.isSnapshot()) {
      return null
    }
    return (
      <a
        href={this.props.teamUpdatesUrl}
        target="_blank"
        rel="noreferrer noopener"
        className={`TopBar-toolsButton`}
      >
        <span>Team Updates</span>
      </a>
    )
  }

  renderSnapshotModal() {
    let highlight = this.props.highlight
    return (
      <button
        onClick={this.props.handleOpenModal}
        className={`TopBar-toolsButton TopBar-createSnapshotButton ${
          highlight ? "isHighlighted" : ""
        }`}
      >
        <SnapshotSvg className="TopBar-snapshotSvg" />
        <span>
          Create a <br />
          Snapshot
        </span>
      </button>
    )
  }

  renderSnapshotOwner() {
    if (this.props.snapshotOwner) {
      return (
        <p className="TopBar-snapshotOwner">
          Snapshot shared by <strong>{this.props.snapshotOwner}</strong>
        </p>
      )
    }
  }
}

export default TopBar
