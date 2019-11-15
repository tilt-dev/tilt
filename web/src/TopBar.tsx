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
          {this.props.showSnapshotButton
            ? this.renderSnapshotModal()
            : this.renderSnapshotOwner()}
        </div>
      </div>
    )
  }

  isSnapshot() {
    return !!this.props.snapshotOwner
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
