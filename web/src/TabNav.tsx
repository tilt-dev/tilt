import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { ResourceView } from "./types"
import "./TabNav.scss"

type NavProps = {
  previewUrl: string
  logUrl: string
  errorsUrl: string
  resourceView: ResourceView
  numberOfErrors: number
}

class TabNav extends PureComponent<NavProps> {
  render() {
    let logIsSelected = this.props.resourceView === ResourceView.Log
    let previewIsSelected = this.props.resourceView === ResourceView.Preview
    let errorsIsSelected = this.props.resourceView === ResourceView.Errors

    // The number of errors should be for the selected resource
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
              className={`tabLink tabLink--errors ${
                errorsIsSelected ? "tabLink--is-selected" : ""
              }`}
              to={this.props.errorsUrl}
            >
              Errors{" "}
              {this.props.numberOfErrors > 0
                ? `(${this.props.numberOfErrors})`
                : ""}
            </Link>
          </li>
        </ul>
      </nav>
    )
  }
}

export default TabNav
