import React, { PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import TimeAgo from "react-timeago"
import "./AlertPane.scss"
import { Resource } from "./types"
import { timeAgoFormatter } from "./timeFormatters"
import { Alert, hasAlert, alertKey } from "./alerts"

type AlertsProps = {
  resources: Array<Resource>
  handleSendAlert: (alert: Alert) => void
  teamAlertsIsEnabled: boolean
  alertLinks: { [key: string]: string }
}

function logToLines(s: string) {
  return s.split("\n").map((l, i) => <AnsiLine key={"logLine" + i} line={l} />)
}

function renderAlertLinkButton(
  alert: Alert,
  alertLinks: { [key: string]: string },
  handleSendAlert: (alert: Alert) => void
) {
  let key = alertKey(alert)
  let hasLink = alertLinks.hasOwnProperty(key)

  if (!hasLink) {
    return (
      <section className="AlertPane-headerDiv-alertUrlWrap">
        <button onClick={() => handleSendAlert(alert)}>Get Link</button>
      </section>
    )
  } else {
    return (
      <section className="AlertPane-headerDiv-alertUrlWrap">
        <p className="AlertPane-headerDiv-alertUrl">{alertLinks[key]}</p>
        <button
          title="Open link in new tab"
          onClick={() => window.open(alertLinks[key])}
        >
          Open
        </button>
      </section>
    )
  }
}

function renderAlerts(
  resources: Array<Resource>,
  teamAlertsIsEnabled: boolean,
  alertLinks: { [key: string]: string },
  handleSendAlert: (alert: Alert) => void
) {
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
              {teamAlertsIsEnabled &&
                renderAlertLinkButton(alert, alertLinks, handleSendAlert)}
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

    let alerts = renderAlerts(
      this.props.resources,
      this.props.teamAlertsIsEnabled,
      this.props.alertLinks,
      this.props.handleSendAlert
    )
    if (alerts.length > 0) {
      el = <ul>{alerts}</ul>
    }

    return <section className="AlertPane">{el}</section>
  }
}

export default AlertPane
