import React, { PureComponent } from "react"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import { Link } from "react-router-dom"
import { combinedStatus, warnings } from "./status"
import "./Sidebar.scss"
import { ResourceView } from "./types"

class SidebarItem {
  name: string
  status: string
  hasWarnings: boolean
  hasEndpoints: boolean

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: any) {
    this.name = res.Name
    this.status = combinedStatus(res)
    this.hasWarnings = warnings(res).length > 0
    this.hasEndpoints = (res.Endpoints || []).length
  }
}

type SidebarProps = {
  isClosed: boolean
  items: SidebarItem[]
  selected: string
  toggleSidebar: any
  resourceView: ResourceView
}

class Sidebar extends PureComponent<SidebarProps> {
  render() {
    let classes = ["Sidebar"]
    if (this.props.isClosed) {
      classes.push("is-closed")
    }

    let allItemClasses = "resLink resLink--all"
    if (!this.props.selected) {
      allItemClasses += " is-selected"
    }
    let allItem = (
      <li>
        <Link className={allItemClasses} to="/">
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
      }
      let classes = `resLink resLink--${item.status}`
      if (this.props.selected === item.name) {
        classes += " is-selected"
      }
      if (item.hasWarnings) {
        classes += " has-warnings"
      }
      return (
        <li key={item.name}>
          <Link className={classes} to={link}>
            {item.name}
          </Link>
        </li>
      )
    })

    let logResourceViewURL = this.props.selected
      ? `/r/${this.props.selected}`
      : "/"
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
        <button className="Sidebar-toggle" onClick={this.props.toggleSidebar}>
          <ChevronSvg /> Collapse
        </button>
      </section>
    )
  }
}

export default Sidebar

export { SidebarItem }
