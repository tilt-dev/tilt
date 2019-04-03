import React, { PureComponent } from "react"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import { Link } from "react-router-dom"
import { combinedStatus } from "./status"
import "./Sidebar.scss"
import { ResourceView } from "./HUD"

class SidebarItem {
  name: string
  status: string

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: any) {
    this.name = res.Name
    this.status = combinedStatus(res)
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
          &nbsp;ALL
        </Link>
      </li>
    )

    let listItems = this.props.items.map(item => {
      let link = `/r/${item.name}`
      if (this.props.resourceView === ResourceView.Preview) {
        link += "/preview"
      }
      let classes = `resLink resLink--${item.status}`
      if (this.props.selected === item.name) {
        classes += " is-selected"
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
    let previewResourceViewURL = this.props.selected
      ? `/r/${this.props.selected}/preview`
      : "/preview"

    return (
      <div className={classes.join(" ")}>
        <nav className="Sidebar-main">
          <h2 className="Sidebar-header">RESOURCES:</h2>
          <ul className="Sidebar-list">
            {allItem}
            {listItems}
          </ul>
          <button className="Sidebar-toggle" onClick={this.props.toggleSidebar}>
            <ChevronSvg /> Collapse
          </button>
        </nav>
        <nav className="Sidebar-secondary">
          <Link to={logResourceViewURL}>Logs</Link>
          <Link to={previewResourceViewURL}>Preview</Link>
        </nav>
      </div>
    )
  }
}

export default Sidebar

export { SidebarItem }
