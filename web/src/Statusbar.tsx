import React, { PureComponent, ReactElement } from "react"
import { ReactComponent as LogoSvg } from "./assets/svg/logo.svg"
import { ReactComponent as ErrorSvg } from "./assets/svg/error.svg"
import { ReactComponent as WarningSvg } from "./assets/svg/warning.svg"
import { ReactComponent as UpdateAvailableSvg } from "./assets/svg/update-available.svg"
import { combinedStatus, warnings } from "./status"
import "./Statusbar.scss"
import { combinedStatusMessage } from "./combinedStatusMessage"
import { Build, TiltBuild } from "./types"
import mostRecentBuildToDisplay from "./mostRecentBuild"
import { Link } from "react-router-dom"

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
    this.name = res.Name
    this.warningCount = warnings(res).length

    let status = combinedStatus(res)
    this.up = status === "ok"
    this.hasError = status === "error"
    this.currentBuild = res.CurrentBuild
    this.buildHistory = res.BuildHistory
    this.lastBuild = res.BuildHistory ? res.BuildHistory[0] : null
    this.podStatus = res.K8sResourceInfo && res.K8sResourceInfo.PodStatus
    this.podStatusMessage =
      res.K8sResourceInfo && res.K8sResourceInfo.PodStatusMessage
    this.pendingBuildSince = res.PendingBuildSince
    this.pendingBuildEdits = res.PendingBuildEdits
  }
}

type StatusBarProps = {
  items: Array<StatusItem>
  alertsUrl: string
  runningVersion: TiltBuild | null
  latestVersion: TiltBuild | null
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

  statusMessagePanel(build: any, editMessage: string) {
    return (
      <section className="Statusbar-panel Statusbar-statusMsgPanel">
        <p className="Statusbar-statusMsgPanel-child">
          {combinedStatusMessage(this.props.items)}
        </p>
        <p className="Statusbar-statusMsgPanel-child Statusbar-statusMsgPanel-child--lastEdit">
          <span className="Statusbar-statusMsgPanel-label">Last Edit:</span>
          <span>{build ? editMessage : "—"}</span>
        </p>
      </section>
    )
  }

  tiltPanel(runningVersion: TiltBuild | null, latestVersion: TiltBuild | null) {
    let content: ReactElement = <LogoSvg className="Statusbar-logo" />
    if (
      latestVersion &&
      latestVersion.Version &&
      runningVersion &&
      runningVersion.Version &&
      !runningVersion.Dev &&
      runningVersion.Version !== latestVersion.Version
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
            Get Tilt v{latestVersion.Version}!{" "}
            <span role="img" aria-label="Decorative sparkling stars">
              ✨
            </span>
            <br />
            (You're running v{runningVersion.Version})
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
      editMessage = `${build.name} ‣ ${build.edits[0]}`
      if (build.edits.length > 1) {
        editMessage += `[+${build.edits.length - 1} more]`
      }
    }
    let statusMessagePanel = this.statusMessagePanel(build, editMessage)

    let resCount = items.length
    let progressPanel = this.progressPanel(upCount, resCount)

    let tiltPanel = this.tiltPanel(
      this.props.runningVersion,
      this.props.latestVersion
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
