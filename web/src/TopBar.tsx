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
            this.props.handleSendSnapshot
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
  handleSendSnapshot: (snapshot: Snapshot) => void
) {
  //TODO TFT - formatting the button
  return <button onClick={() => handleSendSnapshot(snapshot)}>Get Link</button>
}

export default TopBar
