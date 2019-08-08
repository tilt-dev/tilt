import React, { PureComponent } from "react"
import { ResourceView, Snapshot } from "./types"
import "./TopBar.scss"
import SailInfo from "./SailInfo"
import TabNav from "./TabNav"
import { Alert } from "./alerts"

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
  //TODO TFT: add snapshot feature flag boolean
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
        {/*//TODO TFT: check for snapshot feature flag boolean */}
        <div className="TopBar-headerDiv-snapshotURL">
          {renderSnapshotLinkButton(
            this.props.state,
            this.props.handleSendSnapshot,
            this.props.snapshotURL
          )}
        </div>
        <span className="TopBar-spacer">&nbsp;</span>
        <SailInfo
          sailEnabled={this.props.sailEnabled}
          sailUrl={this.props.sailUrl}
        />
      </div>
    )
  }
}

function renderSnapshotLinkButton(
  snapshot: Snapshot,
  handleSendSnapshot: (snapshot: Snapshot) => void,
  snapshotURL: string
) {
  let hasLink = snapshotURL != ""
  if (!hasLink) {
    return (
      <section className="TopBar-headerDiv-snapshotUrlWrap">
        <button onClick={() => handleSendSnapshot(snapshot)}>Get Link</button>
      </section>
    )
  } else {
    return (
      <section className="TopBar-headerDiv-snapshotUrlWrap">
        <p className="TopBar-headerDiv-snapshotUrl">{snapshotURL}</p>
        <button
          title="Open link in new tab"
          onClick={() => window.open(snapshotURL)}
        >
          Open
        </button>
      </section>
    )
  }
  //TODO TFT - formatting the button
}

export default TopBar
