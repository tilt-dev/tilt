import React, { PureComponent } from 'react';
import { ReactComponent as LogoSvg } from './assets/svg/logo-imagemark.svg';
import { isZeroTime } from './time';
import './Statusbar.scss';

const nbsp = '\u00a0'

class StatusItem {
  public warnings: Array<string> = []
  public up: boolean = false
  public error: string = ""
  public name: string

  /**
   * Create a pared down StatusItem from a ResourceView
   */
  constructor(res: any) {
    this.name = res.Name

    let buildHistory = res.BuildHistory || []
    let lastBuild = buildHistory[0]
    this.warnings = (lastBuild && lastBuild.Warnings) || []

    let runtimeStatus = res.RuntimeStatus
    let currentBuild = res.CurrentBuild
    let hasCurrentBuild = Boolean(currentBuild && !isZeroTime(currentBuild.StartTime))
    let hasPendingBuild = !isZeroTime(res.PendingBuildSince)
    let lastBuildError: string = lastBuild ? lastBuild.Error : ''

    this.up = Boolean(runtimeStatus === "ok" && !hasCurrentBuild && !lastBuildError && !hasPendingBuild)

    this.error = runtimeStatus === "error" ? lastBuildError : ''
  }
}

type StatusBarProps = {
  items: Array<any>
  toggleSidebar: any
}

class Statusbar extends PureComponent<StatusBarProps> {
  errorPanel(errorCount: number) {
    let errorPanelClasses = 'Statusbar-panel Statusbar-panel--error'
    let icon = <span role="img" className="icon" aria-label="Error">{errorCount > 0 ? '❌' : nbsp}</span>
    let message = <span>{errorCount} {errorCount === 1 ? 'Error' : 'Errors'}</span>
    return (<div className={errorPanelClasses}>{icon}&nbsp;{message}</div>)
  }

  warningPanel(warningCount: number) {
    let warningPanelClasses = 'Statusbar-panel Statusbar-panel--warning'
    let icon = <span role="img" className="icon" aria-label="Warning">{warningCount > 0 ? '▲' : nbsp}</span>
    let message = <span>{warningCount} {warningCount === 1 ? 'Warning' : 'Warnings'}</span>
    return (<div className={warningPanelClasses}>{icon}&nbsp;{message}</div>)
  }

  upPanel(upCount: number, itemCount: number) {
    let upPanelClasses = 'Statusbar-panel Statusbar-panel--up'
    let upPanel = (<button className={upPanelClasses} onClick={this.props.toggleSidebar}>
       {upCount} / {itemCount} resources up <LogoSvg className="icon"/>
    </button>)
    return upPanel
  }

  render() {
    let errorCount = 0
    let warningCount = 0
    let upCount = 0

    let items = this.props.items
    items.forEach((item) => {
      if (item.error) {
        errorCount++
      }
      if (item.up) {
        upCount++
      }
      warningCount += item.warnings.length
    })

    let itemCount = items.length
    let errorPanel = this.errorPanel(errorCount)
    let warningPanel = this.warningPanel(warningCount)
    let upPanel = this.upPanel(upCount, itemCount)

    return (<div className="Statusbar">
      {errorPanel}
      {warningPanel}
      <div className="Statusbar-panel Statusbar-panel--spacer">&nbsp;</div>
      {upPanel}
    </div>)
  }
}

export default Statusbar;

export {StatusItem};
