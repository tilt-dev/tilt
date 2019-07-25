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
  featureFlags: { [featureFlag: string]: boolean }
}

function logToLines(s: string) {
  return s.split("\n").map((l, i) => <AnsiLine key={"logLine" + i} line={l} />)
}

function alertElements(
  resources: Array<Resource>,
  handleSendAlert: (alert: Alert) => void,
  featureFlags: { [featureFlag: string]: boolean }
) {
  let formatter = timeAgoFormatter
  let alertElements: Array<JSX.Element> = []

  resources.forEach(resource => {
    resource.Alerts.forEach(alert => {
      alertElements.push(
        <li key={alert.alertType + resource.Name} className="AlertPane-item">
          <header>
            <p>{resource.Name}</p>
            {alert.titleMsg != "" && <p>{alert.titleMsg}</p>}
            <p>
              <TimeAgo date={alert.timestamp} formatter={formatter} />
            </p>
          </header>
          <section>{logToLines(alert.msg)}</section>
          {!featureFlags ||
            (featureFlags && featureFlags.team_alerts && (
              <footer>
                <button onClick={() => handleSendAlert(alert)}>
                  Get Alert Link
                </button>
              </footer>
            ))}
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
      this.props.featureFlags
    )
    if (alerts.length > 0) {
      el = <ul>{alerts}</ul>
    }

    return <section className="AlertPane">{el}</section>
  }
}

export default AlertPane
