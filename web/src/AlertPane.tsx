import React, { PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import TimeAgo from "react-timeago"
import "./AlertPane.scss"
import { Resource } from "./types"
import { timeAgoFormatter } from "./timeFormatters"
import { Alert, hasAlert } from "./alerts"

type AlertsProps = {
  resources: Array<Resource>
}

function logToLines(s: string) {
  return s.split("\n").map((l, i) => <AnsiLine key={"logLine" + i} line={l} />)
}

function renderAlerts(resources: Array<Resource>) {
  let formatter = timeAgoFormatter
  let alertElements: Array<JSX.Element> = []

  let alertResources = resources.filter(r => hasAlert(r))
  alertResources.forEach(resource => {
    resource.Alerts.forEach(alert => {
      alertElements.push(
        <li key={alert.alertType + resource.Name} className="AlertPane-item">
          <header>
            <div className="AlertPane-headerDiv">
              <h3 className="AlertPane-headerDiv-header">{alert.header}</h3>
            </div>
            <div className="AlertPane-headerDiv">
              <p>
                <span>Resource: {alert.resourceName}</span>
                <span>Type: {alert.alertType}</span>
              </p>
              <TimeAgo date={alert.timestamp} formatter={formatter} />
            </div>
          </header>
          <section>{logToLines(alert.msg)}</section>
        </li>
      )
    })
  })
  return alertElements
}

class AlertPane extends PureComponent<AlertsProps> {
  render() {
    let el = (
      <section className="Pane-empty-message">
        <LogoWordmarkSvg />
        <h2>No Alerts Found</h2>
      </section>
    )

    let alerts = renderAlerts(this.props.resources)
    if (alerts.length > 0) {
      el = <ul>{alerts}</ul>
    }

    return <section className="AlertPane">{el}</section>
  }
}

export default AlertPane
