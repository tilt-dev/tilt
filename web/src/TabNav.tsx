import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { SidebarItem } from "./Sidebar"
import { ResourceView } from "./HUD"
import "./TabNav.scss"
type NavProps = {
  resourceName: string
  sidebarItems: Array<SidebarItem>
  resourceView: ResourceView
}

class TabNav extends PureComponent<NavProps> {
  render() {
    let name = this.props.resourceName
    let logResourceViewURL = name === "" ? "/" : `/r/${name}`
    let previewResourceURL = "/"
    if (name) {
      previewResourceURL = `/r/${name}/preview`
    } else if (this.props.sidebarItems.length) {
      // Pick the first item with an endpoint, or default to the first item
      previewResourceURL = `/r/${this.props.sidebarItems[0].name}/preview`
      this.props.sidebarItems.forEach(r => {
        if (r.hasEndpoints) {
          previewResourceURL = `/r/${r.name}/preview`
          return
        }
      })
    }
    let logIsSelected = this.props.resourceView == ResourceView.Log
    let previewIsSelected = this.props.resourceView == ResourceView.Preview
    return (
      <nav className="TabNav">
        <Link
          className={logIsSelected ? "viewLink--is-selected" : ""}
          to={logResourceViewURL}
        >
          Logs
        </Link>
        <Link
          className={previewIsSelected ? "viewLink--is-selected" : ""}
          to={previewResourceURL}
        >
          Preview
        </Link>
      </nav>
    )
  }
}

export default TabNav
