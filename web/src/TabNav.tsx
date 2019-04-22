import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { ResourceView } from "./types"
import "./TabNav.scss"

type NavProps = {
  previewUrl: string
  logUrl: string
  errorsUrl: string
  resourceView: ResourceView
}

class TabNav extends PureComponent<NavProps> {
  render() {
    let logIsSelected = this.props.resourceView == ResourceView.Log
    let previewIsSelected = this.props.resourceView == ResourceView.Preview
    let errorsIsSelected = this.props.resourceView == ResourceView.Errors
    return (
      <nav className="TabNav">
        <ul>
          <li>
            <Link
              className={`tabLink ${
                logIsSelected ? "tabLink--is-selected" : ""
              }`}
              to={this.props.logUrl}
            >
              Logs
            </Link>
          </li>
          <li>
            <Link
              className={`tabLink ${
                previewIsSelected ? "tabLink--is-selected" : ""
              }`}
              to={this.props.previewUrl}
            >
              Preview
            </Link>
          </li>
          <li>
            <Link
              className={`tabLink ${
                errorsIsSelected ? "tabLink--is-selected" : ""
              }`}
              to={this.props.errorsUrl}
            >
              Errors
            </Link>
          </li>
        </ul>
      </nav>
    )
  }
}

export default TabNav
