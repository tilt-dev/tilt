import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { ResourceView } from "./types"
import "./TabNav.scss"

type NavProps = {
  previewUrl: string
  logUrl: string
  resourceView: ResourceView
}

class TabNav extends PureComponent<NavProps> {
  render() {
    let logIsSelected = this.props.resourceView == ResourceView.Log
    let previewIsSelected = this.props.resourceView == ResourceView.Preview
    return (
      <nav className="TabNav">
        <Link
          className={logIsSelected ? "viewLink--is-selected" : ""}
          to={this.props.logUrl}
        >
          Logs
        </Link>
        <Link
          className={previewIsSelected ? "viewLink--is-selected" : ""}
          to={this.props.previewUrl}
        >
          Preview
        </Link>
      </nav>
    )
  }
}

export default TabNav
