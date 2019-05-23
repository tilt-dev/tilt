import React, { PureComponent } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import TimeAgo from "react-timeago"
import "./AlertPane.scss"
import { zeroTime } from "./time"
import { Build } from "./types"
import { timeAgoFormatter } from "./timeFormatters"

class AlertResource {
  public name: string
  public buildHistory: Array<Build>
  public resourceInfo: ResourceInfo
  public crashLog: string

  constructor(resource: any) {
    this.name = resource.Name
    this.buildHistory = resource.BuildHistory
    this.crashLog = resource.CrashLog
    if (resource.ResourceInfo) {
      this.resourceInfo = {
        podCreationTime: resource.ResourceInfo.PodCreationTime,
        podStatus: resource.ResourceInfo.PodStatus,
        podRestarts: resource.ResourceInfo.PodRestarts,
      }
    } else {
      this.resourceInfo = {
        podCreationTime: zeroTime,
        podStatus: "",
        podRestarts: 0,
      }
    }
  }

  public hasAlert() {
    return this.podStatusIsError() || this.podRestarted() || this.buildFailed()
  }

  public crashRebuild() {
    return this.buildHistory.length > 0 && this.buildHistory[0].IsCrashRebuild
  }

  public podStatusIsError() {
    return (
      this.resourceInfo.podStatus === "Error" ||
      this.resourceInfo.podStatus === "CrashLoopBackOff"
    )
  }

  public podRestarted() {
    return this.resourceInfo.podRestarts > 0
  }

  public buildFailed() {
    return this.buildHistory.length > 0 && this.buildHistory[0].Error !== null
  }

  public numberOfAlerts(): number {
    let num = 0
    if (this.podStatusIsError() || this.podRestarted() || this.crashRebuild()) {
      num++
    }
    if (this.buildFailed()) {
      num++
    }

    return num
  }
}

type ResourceInfo = {
  podCreationTime: string
  podStatus: string
  podRestarts: number
}

type AlertsProps = {
  resources: Array<AlertResource>
}

function logToLines(s: string) {
  return s.split("\n").map((l, i) => <AnsiLine key={"logLine" + i} line={l} />)
}

class AlertPane extends PureComponent<AlertsProps> {
  render() {
    let formatter = timeAgoFormatter
    let el = (
      <section className="Pane-empty-message">
        <LogoWordmarkSvg />
        <h2>No Alerts Found</h2>
      </section>
    )
    let errorElements: Array<JSX.Element> = []
    this.props.resources.forEach(r => {
      if (r.podStatusIsError()) {
        errorElements.push(
          <li key={"resourceInfoError" + r.name} className="AlertPane-item">
            <header>
              <p>{r.name}</p>
              <p>
                <TimeAgo
                  date={r.resourceInfo.podCreationTime}
                  formatter={formatter}
                />
              </p>
            </header>
            <section>{logToLines(r.crashLog)}</section>
          </li>
        )
      } else if (r.podRestarted()) {
        errorElements.push(
          <li key={"resourceInfoPodCrash" + r.name} className="AlertPane-item">
            <header>
              <p>{r.name}</p>
              <p>{`Restarts: ${r.resourceInfo.podRestarts}`}</p>
            </header>
            <section>{logToLines(r.crashLog)}</section>
          </li>
        )
      } else if (r.crashRebuild()) {
        errorElements.push(
          <li
            key={"resourceInfoCrashRebuild" + r.name}
            className="AlertPane-item"
          >
            <header>
              <p>{r.name}</p>
              <p>Pod crashed!</p>
            </header>
            <section>{logToLines(r.crashLog)}</section>
          </li>
        )
      }
      if (r.buildFailed()) {
        let lastBuild = r.buildHistory[0]
        errorElements.push(
          <li key={"buildError" + r.name} className="AlertPane-item">
            <header>
              <p>{r.name}</p>
              <p>
                <TimeAgo date={lastBuild.FinishTime} formatter={formatter} />
              </p>
            </header>
            <section>{logToLines(lastBuild.Log)}</section>
          </li>
        )
      }
    })

    if (errorElements.length > 0) {
      el = <ul>{errorElements}</ul>
    }

    return <section className="AlertPane">{el}</section>
  }
}

export default AlertPane
export { AlertResource }
