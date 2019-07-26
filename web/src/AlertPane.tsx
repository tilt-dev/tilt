import React, { PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import TimeAgo from "react-timeago"
import "./AlertPane.scss"
import { zeroTime } from "./time"
import { Build, Resource, ResourceInfo } from "./types"
import { timeAgoFormatter } from "./timeFormatters"
import { podStatusIsCrash, podStatusIsError } from "./constants"
import { Alert } from "./alerts"

type AlertsProps = {
  resources: Array<Resource>
  handleSendAlert: (alert: Alert) => void
  teamAlertsIsEnabled: boolean
}

function logToLines(s: string) {
  return s.split("\n").map((l, i) => <AnsiLine key={"logLine" + i} line={l} />)
}

function alertElements(
  resources: Array<Resource>,
  handleSendAlert: (alert: Alert) => void,
  teamAlertsIsEnabled: boolean
) {
  let formatter = timeAgoFormatter
  let alertElements: Array<JSX.Element> = []

  resources.forEach(resource => {
    resource.Alerts.forEach(alert => {
      alertElements.push(
        <li key={alert.alertType + resource.Name} className="AlertPane-item">
          <header>
            <p>{resource.Name}</p>
            {alert.header != "" && <p>{alert.header}</p>}
            <time>
              <TimeAgo date={alert.timestamp} formatter={formatter} />
            </time>
          </header>
          <section>{logToLines(alert.msg)}</section>
          {teamAlertsIsEnabled && (
            <footer>
              <button onClick={() => handleSendAlert(alert)}>
                Get Alert Link
              </button>
            </footer>
          )}
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

    let alerts = alertElements(
      this.props.resources,
      this.props.handleSendAlert,
      this.props.teamAlertsIsEnabled
    )
    if (alerts.length > 0) {
      el = <ul>{alerts}</ul>
    }

    return <section className="AlertPane">{el}</section>
  }
}

export default AlertPane
