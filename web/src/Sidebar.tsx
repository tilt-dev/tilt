import React, { PureComponent } from "react"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import { ReactComponent as DotSvg } from "./assets/svg/dot.svg"
import { ReactComponent as DotBuildingSvg } from "./assets/svg/dot-building.svg"
import { Link } from "react-router-dom"
import { combinedStatus, warnings } from "./status"
import "./Sidebar.scss"
import { ResourceView } from "./types"
import TimeAgo from "react-timeago"
import { isZeroTime } from "./time"
import PathBuilder from "./PathBuilder"
import { timeAgoFormatter } from "./timeFormatters"
import { AlertResource } from "./ErrorPane"

class SidebarItem {
  name: string
  status: string
  hasWarnings: boolean
  hasEndpoints: boolean
  lastDeployTime: string
  pendingBuildSince: string
  currentBuildStartTime: string
  alertResource: AlertResource

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: any) {
    this.name = res.Name
    this.status = combinedStatus(res)
    this.hasWarnings = warnings(res).length > 0
    this.hasEndpoints = (res.Endpoints || []).length
    this.lastDeployTime = res.LastDeployTime
    this.pendingBuildSince = res.PendingBuildSince
    this.currentBuildStartTime = res.CurrentBuild.StartTime
    this.alertResource = new AlertResource(res)
  }

  numberOfAlerts(): number {
    return this.alertResource.numberOfAlerts()
  }
}

type SidebarProps = {
  isClosed: boolean
  items: SidebarItem[]
  selected: string
  toggleSidebar: any
  resourceView: ResourceView
  pathBuilder: PathBuilder
}

class Sidebar extends PureComponent<SidebarProps> {
  render() {
    let pb = this.props.pathBuilder
    let classes = ["Sidebar"]
    if (this.props.isClosed) {
      classes.push("is-closed")
    }

    let allItemClasses = "resLink resLink--all"
    if (!this.props.selected) {
      allItemClasses += " is-selected"
    }
    let allLink =
      this.props.resourceView === ResourceView.Alerts
        ? pb.path("/alerts")
        : pb.path("/")
    let allItem = (
      <li>
        <Link className={allItemClasses} to={allLink}>
          All
        </Link>
      </li>
    )

    let listItems = this.props.items.map(item => {
      let link = `/r/${item.name}`
      if (this.props.resourceView === ResourceView.Preview) {
        link += "/preview"
      } else if (this.props.resourceView === ResourceView.Alerts) {
        link += "/alerts"
      }

      let formatter = timeAgoFormatter
      let hasBuilt = !isZeroTime(item.lastDeployTime)
      let willBuild = !isZeroTime(item.pendingBuildSince)
      let building = !isZeroTime(item.currentBuildStartTime)
      let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />

      let classes = `resLink resLink--${
        willBuild || building ? "building" : item.status
      }`
      if (this.props.selected === item.name) {
        classes += " is-selected"
      }
      if (item.hasWarnings) {
        classes += " has-warnings"
      }

      return (
        <li key={item.name}>
          <Link className={classes} to={pb.path(link)}>
            <span className="resLink-icon">
              {willBuild || building ? <DotBuildingSvg /> : <DotSvg />}
            </span>
            <span className="resLink-name">{item.name}</span>
            {item.numberOfAlerts() > 0 ? (
              <span className="resLink-alertNum">{item.numberOfAlerts()}</span>
            ) : (
              ""
            )}
            <span>{hasBuilt ? timeAgo : ""}</span>
          </Link>
        </li>
      )
    })

    return (
      <section className={classes.join(" ")}>
        <nav className="Sidebar-resources">
          <ul className="Sidebar-list">
            {allItem}
            {listItems}
          </ul>
        </nav>
        <div className="Sidebar-spacer">&nbsp;</div>
        <button className="Sidebar-toggle" onClick={this.props.toggleSidebar}>
          <ChevronSvg /> Collapse
        </button>
      </section>
    )
  }
}

export default Sidebar

export { SidebarItem }
