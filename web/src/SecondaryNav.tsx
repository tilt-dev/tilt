import React, { PureComponent } from "react"
import { ResourceView, SnapshotHighlight } from "./types"
import { ReactComponent as SnapshotSvg } from "./assets/svg/snapshot.svg"
// import "./SecondaryNav.scss"
import SecondaryNavTabs from "./SecondaryNavTabs"

type SecondaryNavProps = {
  logUrl: string
  alertsUrl: string
  resourceView: ResourceView
  numberOfAlerts: number
  facetsUrl: string | null
}

class SecondaryNav extends PureComponent<SecondaryNavProps> {
  render() {
    return (
      <div className="SecondaryNav">
        <SecondaryNavTabs
          logUrl={this.props.logUrl}
          alertsUrl={this.props.alertsUrl}
          facetsUrl={this.props.facetsUrl}
          resourceView={this.props.resourceView}
          numberOfAlerts={this.props.numberOfAlerts}
        />
      </div>
    )
  }
}

export default SecondaryNav
