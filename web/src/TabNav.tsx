import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { ResourceView } from "./types"
import "./TabNav.scss"

type NavProps = {
  previewUrl: string
  logUrl: string
  alertsUrl: string
  resourceView: ResourceView
  numberOfAlerts: number
}

class TabNav extends PureComponent<NavProps> {
  render() {
    let logIsSelected = this.props.resourceView === ResourceView.Log
    let previewIsSelected = this.props.resourceView === ResourceView.Preview
    let alertsIsSelected = this.props.resourceView === ResourceView.Alerts

    // The number of alerts should be for the selected resource
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
                alertsIsSelected ? "tabLink--is-selected" : ""
              }`}
              to={this.props.alertsUrl}
            >
              Alerts
              {this.props.numberOfAlerts > 0 ? (
                <span className="tabLink-alertBadge">
                  {this.props.numberOfAlerts}
                </span>
              ) : (
                ""
              )}
            </Link>
          </li>
        </ul>
      </nav>
    )
  }
}

export default TabNav
