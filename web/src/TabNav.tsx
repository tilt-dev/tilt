import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { ResourceView } from "./types"
import "./TabNav.scss"

type NavProps = {
  previewUrl: string
  logUrl: string
  errorsUrl: string
  resourceView: ResourceView
  sailEnabled: boolean
  sailUrl: string
}

class TabNav extends PureComponent<NavProps> {
  render() {
    let logIsSelected = this.props.resourceView == ResourceView.Log
    let previewIsSelected = this.props.resourceView == ResourceView.Preview
    let errorsIsSelected = this.props.resourceView == ResourceView.Errors

    let spans: Array<JSX.Element> = [
      <span key="TabNav">
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
      </span>,
    ]

    if (this.props.sailEnabled) {
      spans.push(
        <span className="TabNav-spacer" key="spacer">
          &nbsp;
        </span>
      )
      if (this.props.sailUrl) {
        spans.push(
          <span className="sail-url" key="sail-url">
            <a href={this.props.sailUrl}>Share this view!</a>
          </span>
        )
      } else {
        spans.push(
          <span className="sail-url" key="sail-url">
            <button type="button">Share me!</button>
          </span>
        )
      }
    }
    return <nav className="TabNav">{spans}</nav>
  }
}

export default TabNav
