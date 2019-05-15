import React, { PureComponent } from "react"
import { ResourceView } from "./types"
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
        <span className="TopBar-spacer">&nbsp;</span>
        <SailInfo
          sailEnabled={this.props.sailEnabled}
          sailUrl={this.props.sailUrl}
        />
      </div>
    )
  }
}

export default TopBar
