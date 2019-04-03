import React, { PureComponent } from "react"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import { isZeroTime } from "./time"
import { Link } from "react-router-dom"
import "./Sidebar.scss"

class SidebarItem {
  name: string
  status: string

  /**
   * Create a pared down SidebarItem from a ResourceView
   */
  constructor(res: any) {
    this.name = res.Name

    let runtimeStatus = res.RuntimeStatus
    let currentBuild = res.CurrentBuild
    let hasCurrentBuild = Boolean(
      currentBuild && !isZeroTime(currentBuild.StartTime)
    )
    let hasPendingBuild = !isZeroTime(res.PendingBuildSince)

    this.status = runtimeStatus
    if (hasCurrentBuild || hasPendingBuild) {
      this.status = "pending"
    }
  }
}

type SidebarProps = {
  isClosed: boolean
  items: SidebarItem[]
  selected: string
  toggleSidebar: any
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

    return (
      <nav className={classes.join(" ")}>
        <h2 className="Sidebar-header">RESOURCES:</h2>
        <ul className="Sidebar-list">
          {allItem}
          {listItems}
        </ul>
        <button className="Sidebar-toggle" onClick={this.props.toggleSidebar}>
          <ChevronSvg /> Collapse
        </button>
      </nav>
    )
  }
}

export default Sidebar

export { SidebarItem }
