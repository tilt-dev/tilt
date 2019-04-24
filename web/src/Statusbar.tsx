import React, { PureComponent } from "react"
import { ReactComponent as LogoSvg } from "./assets/svg/logo.svg"
import { combinedStatus, warnings } from "./status"
import "./Statusbar.scss"
import { combinedStatusMessage } from "./combinedStatusMessage"
import { Build } from "./types"

const nbsp = "\u00a0"

class StatusItem {
  public warningCount: number = 0
  public up: boolean = false
  public hasError: boolean = false
  public name: string
  public currentBuild: Build
  public lastBuild: Build | null
  public podStatus: string

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
    this.lastBuild = res.BuildHistory ? res.BuildHistory[0] : null
    this.podStatus = res.ResourceInfo && res.ResourceInfo.PodStatus
  }
}

type StatusBarProps = {
  items: Array<StatusItem>
}

class Statusbar extends PureComponent<StatusBarProps> {
  errorPanel(errorCount: number) {
    let errorPanelClasses = "Statusbar-panel Statusbar-panel--error"
    let icon = (
      <span role="img" className="icon" aria-label="Error">
        {errorCount > 0 ? "❌" : nbsp}
      </span>
    )
    let message = (
      <span>
        {errorCount} {errorCount === 1 ? "Error" : "Errors"}
      </span>
    )
    return (
      <div className={errorPanelClasses}>
        {icon}&nbsp;{message}
      </div>
    )
  }

  warningPanel(warningCount: number) {
    let warningPanelClasses = "Statusbar-panel Statusbar-panel--warning"
    let icon = (
      <span role="img" className="icon" aria-label="Warning">
        {warningCount > 0 ? "▲" : nbsp}
      </span>
    )
    let message = (
      <span>
        {warningCount} {warningCount === 1 ? "Warning" : "Warnings"}
      </span>
    )
    return (
      <div className={warningPanelClasses}>
        {icon}&nbsp;{message}
      </div>
    )
  }

  upPanel(upCount: number, itemCount: number) {
    let upPanelClasses = "Statusbar-panel Statusbar-panel--up"
    let upPanel = (
      <span className={upPanelClasses}>
        {upCount} / {itemCount} resources up <LogoSvg className="icon" />
      </span>
    )
    return upPanel
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

    let itemCount = items.length
    let errorPanel = this.errorPanel(errorCount)
    let warningPanel = this.warningPanel(warningCount)
    let upPanel = this.upPanel(upCount, itemCount)

    return (
      <div className="Statusbar">
        {errorPanel}
        {warningPanel}
        <p className="Statusbar-panel--statusMessage">
          {combinedStatusMessage(this.props.items)}
        </p>
        {upPanel}
      </div>
    )
  }
}

export default Statusbar

export { StatusItem }
