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
  state: Snapshot
  handleSendSnapshot: (snapshot: Snapshot) => void
  snapshotURL: string
  showSnapshotButton: boolean
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
          {this.props.showSnapshotButton &&
            renderSnapshotLinkButton(
              this.props.state,
              this.props.handleSendSnapshot,
              this.props.snapshotURL
            )}
          <SailInfo
            sailEnabled={this.props.sailEnabled}
            sailUrl={this.props.sailUrl}
          />
        </section>
      </div>
    )
  }
}

function renderSnapshotLinkButton(
  snapshot: Snapshot,
  handleSendSnapshot: (snapshot: Snapshot) => void,
  snapshotURL: string
) {
  let hasLink = snapshotURL !== ""
  if (!hasLink) {
    return (
      <section className="TopBar-snapshotUrlWrap">
        <button onClick={() => handleSendSnapshot(snapshot)}>
          Share Snapshot
        </button>
      </section>
    )
  } else {
    return (
      <section className="TopBar-snapshotUrlWrap">
        <p className="TopBar-snapshotUrl">{snapshotURL}</p>
        <button
          title="Open link in new tab"
          onClick={() => window.open(snapshotURL)}
        >
          Open
        </button>
      </section>
    )
  }
}

export default TopBar
