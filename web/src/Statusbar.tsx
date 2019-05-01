import React, { PureComponent } from "react"
import { ReactComponent as LogoSvg } from "./assets/svg/logo.svg"
import { ReactComponent as ErrorSvg } from "./assets/svg/error.svg"
import { ReactComponent as WarningSvg } from "./assets/svg/warning.svg"
import { combinedStatus, warnings } from "./status"
import "./Statusbar.scss"
import { combinedStatusMessage } from "./combinedStatusMessage"
import { Build } from "./types"
import mostRecentBuildToDisplay from "./mostRecentBuild"

class StatusItem {
  public warningCount: number = 0
  public up: boolean = false
  public hasError: boolean = false
  public name: string
  public currentBuild: Build
  public lastBuild: Build | null
  public podStatus: string
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
    this.up = status == "ok"
    this.hasError = status == "error"
    this.currentBuild = res.CurrentBuild
    this.buildHistory = res.BuildHistory
    this.lastBuild = res.BuildHistory ? res.BuildHistory[0] : null
    this.podStatus = res.ResourceInfo && res.ResourceInfo.PodStatus
    this.pendingBuildSince = res.PendingBuildSince
    this.pendingBuildEdits = res.PendingBuildEdits
  }
}

type StatusBarProps = {
  items: Array<StatusItem>
}

class Statusbar extends PureComponent<StatusBarProps> {
  errorWarningPanel(errorCount: number, warningCount: number) {
    return (
      <section className="Statusbar-panel err-warn">
        <div className="err-warn-item err-warn-item--error">
          <ErrorSvg className={`icon ${errorCount > 0 ? "icon--error" : ""}`} />
          <p>
            <span className="count">{errorCount}</span> error
            {errorCount === 1 ? "" : "s"}
          </p>
        </div>
        <div className="err-warn-item">
          <WarningSvg
            className={`icon ${warningCount > 0 ? "icon--warning" : ""}`}
          />
          <p>
            <span className="count">{warningCount}</span> warning
            {warningCount === 1 ? "" : "s"}
          </p>
        </div>
      </section>
    )
  }

  progressPanel(upCount: number, itemCount: number) {
    return (
      <section className="Statusbar-panel progress">
        <p>
          <span className="current">{upCount}</span>/{itemCount} running
        </p>
        <LogoSvg className="icon--logo" />
      </section>
    )
  }

  statusMessagePanel(build: any, editMessage: string) {
    return (
      <section className="Statusbar-panel statusMsg">
        <p className="statusMsg-item">
          {combinedStatusMessage(this.props.items)}
        </p>
        <p className="statusMsg-item statusMsg-item--last-edit">
          <span className="label">Last Edit:</span>
          <span className="file">{build ? editMessage : "—"}</span>
        </p>
      </section>
    )
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

    return (
      <div className="Statusbar">
        {errorWarningPanel}
        {statusMessagePanel}
        {progressPanel}
      </div>
    )
  }
}

export default Statusbar

export { StatusItem }
