import React, { PureComponent } from "react"
import { ResourceView } from "./types"
import "./TopBar.scss"
import SailInfo from "./SailInfo"
import TabNav from "./TabNav"

type TopBarProps = {
  previewUrl: string
  logUrl: string
  errorsUrl: string
  resourceView: ResourceView
  sailEnabled: boolean
  sailUrl: string
  numberOfErrors: number
}

class TopBar extends PureComponent<TopBarProps> {
  render() {
    return (
      <div className="TopBar">
        <TabNav
          previewUrl={this.props.previewUrl}
          logUrl={this.props.logUrl}
          errorsUrl={this.props.errorsUrl}
          resourceView={this.props.resourceView}
          numberOfErrors={this.props.numberOfErrors}
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
