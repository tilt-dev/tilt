import React, { PureComponent } from "react"
import { ResourceView, SnapshotHiglight } from "./types"
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
  highlight: SnapshotHiglight | null
}

class TopBar extends PureComponent<TopBarProps> {
  render() {
    return (
      <div className="TopBar">
        <TabNav
          logUrl={this.props.logUrl}
          alertsUrl={this.props.alertsUrl}
          resourceView={this.props.resourceView}
          numberOfAlerts={this.props.numberOfAlerts}
        />
        <section className="TopBar-tools">
          {this.props.showSnapshotButton
            ? this.renderSnapshotModal()
            : this.renderSnapshotOwner()}
        </section>
      </div>
    )
  }

  renderSnapshotModal() {
    let highlight = this.props.highlight
    return (
      <section className="TopBar-snapshotUrlWrap">
        <button onClick={this.props.handleOpenModal}>
          {highlight ? "Share Highlight" : "Share Snapshot"}
        </button>
      </section>
    )
  }

  renderSnapshotOwner() {
    if (this.props.snapshotOwner) {
      return (
        <section className="TopBar-snapshotUrlWrap">
          Snapshot shared by {this.props.snapshotOwner}
        </section>
      )
    } else {
      return null
    }
  }
}

export default TopBar
