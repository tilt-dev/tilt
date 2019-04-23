import React, { PureComponent } from "react"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import { Link } from "react-router-dom"
import { combinedStatus, warnings } from "./status"
import "./Sidebar.scss"
import { ResourceView } from "./types"
import TimeAgo from "react-timeago"
// @ts-ignore
import enStrings from "react-timeago/lib/language-strings/en-short.js"
// @ts-ignore
import buildFormatter from "react-timeago/lib/formatters/buildFormatter"
import { isZeroTime } from "./time"
import PathBuilder from "./PathBuilder"

class SidebarItem {
  name: string
  status: string
  hasWarnings: boolean
  hasEndpoints: boolean
  lastDeployTime: string
  pendingBuildSince: string

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
      this.props.resourceView === ResourceView.Errors
        ? pb.path("/errors")
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
      let analyticsKey = "ui.interactions.logs"
      if (this.props.resourceView === ResourceView.Preview) {
        analyticsKey = "ui.interactions.preview"
        link += "/preview"
      } else if (this.props.resourceView === ResourceView.Errors) {
        analyticsKey = "ui.interactions.errors"
        link += "/errors"
      }
      let classes = `resLink resLink--${item.status}`
      if (this.props.selected === item.name) {
        classes += " is-selected"
      }
      if (item.hasWarnings) {
        classes += " has-warnings"
      }

      let formatter = buildFormatter(enStrings)
      let hasBuilt = !isZeroTime(item.lastDeployTime)
      let timeAgo = <TimeAgo date={item.lastDeployTime} formatter={formatter} />

      return (
        <li key={item.name}>
          <Link className={classes} to={pb.path(link)}>
            <span className="resLink-name">{item.name}</span>
            <span>{hasBuilt ? timeAgo : ""}</span>
          </Link>
        </li>
      )
    })

    let previewResourceViewURL = "/"
    if (this.props.selected) {
      previewResourceViewURL = `/r/${this.props.selected}/preview`
    } else if (this.props.items.length) {
      // Pick the first item with an endpoint, or default to the first item.
      previewResourceViewURL = `/r/${this.props.items[0].name}/preview`

      for (let i = 0; i < this.props.items.length; i++) {
        let item = this.props.items[i]
        if (item.hasEndpoints) {
          previewResourceViewURL = `/r/${item.name}/preview`
          break
        }
      }
    }

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
