import React, { PureComponent, ReactElement } from "react"
import { ReactComponent as LogoSvg } from "./assets/svg/logo.svg"
import { ReactComponent as ErrorSvg } from "./assets/svg/error.svg"
import { ReactComponent as WarningSvg } from "./assets/svg/warning.svg"
import { ReactComponent as UpdateAvailableSvg } from "./assets/svg/update-available.svg"
import { combinedStatus, warnings } from "./status"
import "./Statusbar.scss"
import { combinedStatusMessage } from "./combinedStatusMessage"
import { ResourceStatus } from "./types"
import mostRecentBuildToDisplay from "./mostRecentBuild"
import { Link } from "react-router-dom"

type Build = Proto.webviewBuildRecord
type TiltBuild = Proto.webviewTiltBuild

class StatusItem {
  public warningCount: number = 0
  public up: boolean = false
  public hasError: boolean = false
  public name: string
  public currentBuild: Build
  public lastBuild: Build | null
  public podStatus: string
  public podStatusMessage: string
  public pendingBuildSince: string
  public buildHistory: Array<Build>
  public pendingBuildEdits: Array<string>

  /**
   * Create a pared down StatusItem from a ResourceView
   */
  constructor(res: any) {
    this.name = res.name
    this.warningCount = warnings(res).length

    let status = combinedStatus(res)
    this.up = status === ResourceStatus.Healthy
    this.hasError = status === ResourceStatus.Unhealthy
    this.currentBuild = res.currentBuild
    this.buildHistory = res.buildHistory
    this.lastBuild = res.buildHistory ? res.buildHistory[0] : null
    this.podStatus = res.k8sResourceInfo && res.k8sResourceInfo.podStatus
    this.podStatusMessage =
      res.k8sResourceInfo && res.k8sResourceInfo.podStatusMessage
    this.pendingBuildSince = res.pendingBuildSince
    this.pendingBuildEdits = res.pendingBuildEdits
  }
}

type StatusBarProps = {
  items: Array<StatusItem>
  alertsUrl: string
  runningVersion: TiltBuild | null | undefined
  latestVersion: TiltBuild | null | undefined
  checkVersion: boolean
}

class Statusbar extends PureComponent<StatusBarProps> {
  errorWarningPanel(errorCount: number, warningCount: number) {
    return (
      <section className="Statusbar-panel Statusbar-errWarnPanel">
        <div className="Statusbar-errWarnPanel-child">
          <ErrorSvg
            className={`Statusbar-errWarnPanel-icon ${
              errorCount > 0 ? "Statusbar-errWarnPanel-icon--error" : ""
            }`}
          />
          <p>
            <span className="Statusbar-errWarnPanel-count Statusbar-errWarnPanel-count--error">
              {errorCount}
            </span>{" "}
            <Link to={this.props.alertsUrl}>
              error
              {errorCount === 1 ? "" : "s"}
            </Link>
          </p>
        </div>
        <div className="Statusbar-errWarnPanel-child">
          <WarningSvg
            className={`Statusbar-errWarnPanel-icon ${
              warningCount > 0 ? "Statusbar-errWarnPanel-icon--warning" : ""
            }`}
          />
          <p>
            <span className="Statusbar-errWarnPanel-count">{warningCount}</span>{" "}
            warning
            {warningCount === 1 ? "" : "s"}
          </p>
        </div>
      </section>
    )
  }

  progressPanel(upCount: number, itemCount: number) {
    return (
      <section className="Statusbar-panel Statusbar-progressPanel">
        <p>
          <strong>{upCount}</strong>/{itemCount} running
        </p>
      </section>
    )
  }

  statusMessagePanel(build: any, edits: string) {
    let lastEdit = <span>—</span>
    if (build && edits) {
      lastEdit = (
        <span>
          <span className="LastEditStatus-name" data-tip={edits}>
            {build.name}
          </span>
          <span className="LastEditStatus-details">{" ‣ " + edits}</span>
        </span>
      )
    }
    return (
      <section className="Statusbar-panel Statusbar-statusMsgPanel">
        <div>{combinedStatusMessage(this.props.items)}</div>
        <div className="LastEditStatus">
          <span className="LastEditStatus-label">Last Edit:</span>
          {lastEdit}
        </div>
      </section>
    )
  }

  tiltPanel(
    runningVersion: TiltBuild | null | undefined,
    latestVersion: TiltBuild | null | undefined,
    shouldCheckVersion: boolean
  ) {
    let content: ReactElement = <LogoSvg className="Statusbar-logo" />
    if (
      shouldCheckVersion &&
      latestVersion &&
      latestVersion.version &&
      runningVersion &&
      runningVersion.version &&
      !runningVersion.dev &&
      runningVersion.version !== latestVersion.version
    ) {
      content = (
        <a
          href="https://docs.tilt.dev/upgrade.html"
          className="Statusbar-tiltPanel-link"
          target="_blank"
          rel="noopener noreferrer"
        >
          <p className="Statusbar-tiltPanel-upgradeTooltip">
            <span role="img" aria-label="Decorative sparkling stars">
              ✨
            </span>
            Get Tilt v{latestVersion.version}!{" "}
            <span role="img" aria-label="Decorative sparkling stars">
              ✨
            </span>
            <br />
            (You're running v{runningVersion.version})
          </p>
          {content}
          <UpdateAvailableSvg />
        </a>
      )
    }

    return <section className="Statusbar-tiltPanel">{content}</section>
  }

  render() {
    let errorCount = 0
    let warningCount = 0
    let upCount = 0

    let items = this.props.items
    items.forEach(item => {
      if (item.hasError) {
        errorCount++
      }
      if (item.up) {
        upCount++
      }
      warningCount += item.warningCount
    })
    let errorWarningPanel = this.errorWarningPanel(errorCount, warningCount)

    let build = mostRecentBuildToDisplay(this.props.items)
    let editMessage = ""
    if (build && build.edits.length > 0) {
      editMessage = `${build.edits[0]}`
      if (build.edits.length > 1) {
        editMessage += `[+ ${build.edits.length - 1} more]`
      }
    }
    let statusMessagePanel = this.statusMessagePanel(build, editMessage)

    let resCount = items.length
    let progressPanel = this.progressPanel(upCount, resCount)

    let tiltPanel = this.tiltPanel(
      this.props.runningVersion,
      this.props.latestVersion,
      this.props.checkVersion
    )

    return (
      <div className="Statusbar">
        {errorWarningPanel}
        {statusMessagePanel}
        {progressPanel}
        {tiltPanel}
      </div>
    )
  }
}

export default Statusbar

export { StatusItem }
