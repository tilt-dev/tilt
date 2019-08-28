import React, { PureComponent } from "react"
import { ResourceView, Snapshot } from "./types"
import "./TopBar.scss"
import SailInfo from "./SailInfo"
import TabNav from "./TabNav"

type TopBarProps = {
  previewUrl: string
  logUrl: string
  alertsUrl: string
  resourceView: ResourceView
  sailEnabled: boolean
  sailUrl: string
  numberOfAlerts: number
  showSnapshotButton: boolean
  snapshotOwner: string | null
  handleOpenModal: () => void
}

class TopBar extends PureComponent<TopBarProps> {
  render() {
    return (
      <div className="TopBar">
        <TabNav
          previewUrl={this.props.previewUrl}
          logUrl={this.props.logUrl}
          alertsUrl={this.props.alertsUrl}
          resourceView={this.props.resourceView}
          numberOfAlerts={this.props.numberOfAlerts}
        />
        <section className="TopBar-tools">
          {this.props.showSnapshotButton
            ? this.renderSnapshotModal()
            : this.renderSnapshotOwner()}
          <SailInfo
            sailEnabled={this.props.sailEnabled}
            sailUrl={this.props.sailUrl}
          />
        </section>
      </div>
    )
  }

  renderSnapshotModal() {
    return (
      <section className="TopBar-snapshotUrlWrap">
        <button onClick={this.props.handleOpenModal}>Share Snapshot</button>
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
