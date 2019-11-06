import React, { PureComponent } from "react"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import { Link } from "react-router-dom"
import { combinedStatus, warnings } from "./status"
import "./Sidebar.scss"
import {
  ResourceView,
  TriggerMode,
  RuntimeStatus,
  Build,
  Resource,
} from "./types"
import TimeAgo from "react-timeago"
import { isZeroTime } from "./time"
import PathBuilder from "./PathBuilder"
import { timeAgoFormatter } from "./timeFormatters"
import SidebarIcon from "./SidebarIcon"
import SidebarTriggerButton from "./SidebarTriggerButton"
import { numberOfAlerts } from "./alerts"

class SidebarItem {
  name: string
  status: RuntimeStatus
  hasWarnings: boolean
  hasEndpoints: boolean
  lastDeployTime: string
  pendingBuildSince: string
  currentBuildStartTime: string
  alertCount: number
  triggerMode: TriggerMode
  hasPendingChanges: boolean
  queued: boolean
  lastBuild: Build | null = null

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: Resource) {
    this.name = res.name
    this.status = combinedStatus(res)
    this.hasWarnings = warnings(res).length > 0
    this.hasEndpoints = (res.endpoints || []).length > 0
    this.lastDeployTime = res.lastDeployTime
    this.pendingBuildSince = res.pendingBuildSince
    this.currentBuildStartTime = res.currentBuild.startTime
    this.alertCount = numberOfAlerts(res)
    this.triggerMode = res.triggerMode
    this.hasPendingChanges = res.hasPendingChanges
    this.queued = res.queued
    let buildHistory = res.buildHistory || []
    if (buildHistory.length > 0) {
      this.lastBuild = buildHistory[0]
    }
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

    let allItemClasses = "SidebarItem SidebarItem--all"
    if (!this.props.selected) {
      allItemClasses += " is-selected"
    }
    let allLink =
      this.props.resourceView === ResourceView.Alerts
        ? pb.path("/alerts")
        : pb.path("/")
    let totalAlerts = this.props.items
      .map(i => i.alertCount)
      .reduce((sum, current) => sum + current, 0)

    let allItem = (
      <li className={allItemClasses}>
        <Link className="SidebarItem-link" to={allLink} title="All">
          <div className="SidebarItem-allIcon">┌</div>
          <span className="SidebarItem-name">All</span>
          {totalAlerts > 0 ? (
            <span className="SidebarItem-alertBadge">{totalAlerts}</span>
          ) : (
            ""
          )}
        </Link>
      </li>
    )

    let listItems = this.props.items.map(item => {
      let link = `/r/${item.name}`
      switch (this.props.resourceView) {
        case ResourceView.Alerts:
          link += "/alerts"
          break
        case ResourceView.Facets:
          link += "/facets"
          break
      }

      let formatter = timeAgoFormatter
      let hasBuilt = !isZeroTime(item.lastDeployTime)
      let building = !isZeroTime(item.currentBuildStartTime)
      let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />
      let isSelected = this.props.selected === item.name

      let classes = "SidebarItem"
      if (building) {
        classes += " SidebarItem--building"
      }

      if (isSelected) {
        classes += " is-selected"
      }
      return (
        <li key={item.name} className={classes}>
          <Link
            className="SidebarItem-link"
            to={pb.path(link)}
            title={item.name}
          >
            <SidebarIcon
              status={item.status}
              hasWarning={item.hasWarnings}
              isBuilding={building}
              isDirty={item.hasPendingChanges}
              lastBuild={item.lastBuild}
            />
            <p className="SidebarItem-name">{item.name}</p>
            {item.alertCount > 0 ? (
              <span className="SidebarItem-alertBadge">{item.alertCount}</span>
            ) : (
              ""
            )}
            <span className={`SidebarItem-timeAgo ${hasBuilt ? "" : "empty"}`}>
              {hasBuilt ? timeAgo : "—"}
            </span>
          </Link>
          <SidebarTriggerButton
            isSelected={isSelected}
            hasPendingChanges={item.hasPendingChanges}
            resourceName={item.name}
            hasBuilt={hasBuilt}
            isBuilding={building}
            triggerMode={item.triggerMode}
            isQueued={item.queued}
          />
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
        <button className="Sidebar-toggle" onClick={this.props.toggleSidebar}>
          <ChevronSvg /> Collapse
        </button>
      </section>
    )
  }
}

export default Sidebar

export { SidebarItem }
