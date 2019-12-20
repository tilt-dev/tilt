import React, { PureComponent } from "react"
import { Link } from "react-router-dom"
import { ResourceView } from "./types"
// import "./SecondaryNavTabs.scss"

type NavProps = {
  logUrl: string
  alertsUrl: string
  facetsUrl: string | null
  resourceView: ResourceView
  numberOfAlerts: number
}

class SecondaryNavTabs extends PureComponent<NavProps> {
  render() {
    let logIsSelected = this.props.resourceView === ResourceView.Log
    let alertsIsSelected = this.props.resourceView === ResourceView.Alerts
    let facetsIsSelected = this.props.resourceView === ResourceView.Facets

    // The number of alerts should be for the selected resource
    return (
      <nav className="SecondaryNavTabs">
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
          {this.props.facetsUrl === null ? null : (
            <li>
              <Link
                className={`tabLink ${
                  facetsIsSelected ? "tabLink--is-selected" : ""
                }`}
                to={this.props.facetsUrl}
              >
                Facets
              </Link>
            </li>
          )}
        </ul>
      </nav>
    )
  }
}

export default SecondaryNavTabs
