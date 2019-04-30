import React, { PureComponent } from "react"
import { ResourceView } from "./types"
import "./TopBar.scss"
import SailInfo from "./SailInfo"
import TabNav from "./TabNav"
import PathBuilder from "./PathBuilder"

type TopBarProps = {
  previewUrl: string
  logUrl: string
  errorsUrl: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
  sailEnabled: boolean
  sailUrl: string
}

class TopBar extends PureComponent<TopBarProps> {
  render() {
    let pb = this.props.pathBuilder
    return (
      <div className="TopBar">
        <TabNav
          previewUrl={pb.path(this.props.previewUrl)}
          logUrl={pb.path(this.props.logUrl)}
          errorsUrl={pb.path(this.props.errorsUrl)}
          resourceView={this.props.resourceView}
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
