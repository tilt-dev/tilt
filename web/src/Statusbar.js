import React, { PureComponent } from 'react';
import { ReactComponent as LogoSvg } from './assets/svg/logo-imagemark.svg';
import './Statusbar.scss';

const zeroTime = '0001-01-01T00:00:00Z'
const nbsp = '\u00a0'

function isZeroTime(time) {
  return !time || time === zeroTime
}

class StatusItem {
  /**
   * Create a pared down StatusItem from a ResourceView
   */
  constructor(res) {
    this.name = res.Name

    let buildHistory = res.BuildHistory || []
    let lastBuild = buildHistory[0]
    this.warnings = (lastBuild && lastBuild.Warnings) || []

    let runtimeStatus = res.RuntimeStatus
    let currentBuild = res.CurrentBuild
    let hasCurrentBuild = Boolean(currentBuild && !isZeroTime(currentBuild.StartTime))
    let hasPendingBuild = !isZeroTime(res.PendingBuildSince)

    this.up = Boolean(runtimeStatus === "ok" && !hasCurrentBuild && !lastBuild.Error && !hasPendingBuild)

    this.error = runtimeStatus === "error" || lastBuild.Error || ''
  }
}

class Statusbar extends PureComponent {
  errorPanel(errorCount) {
    let errorPanelClasses = 'Statusbar-panel Statusbar-panel--error'
    let icon = <span role="img" className="icon" aria-label="Error">{errorCount > 0 ? '❌' : nbsp}</span>
    let message = <span>{errorCount} {errorCount === 1 ? 'Error' : 'Errors'}</span>
    return (<div className={errorPanelClasses}>{icon}&nbsp;{message}</div>)
  }

  warningPanel(warningCount) {
    let warningPanelClasses = 'Statusbar-panel Statusbar-panel--warning'
    let icon = <span role="img" className="icon" aria-label="Warning">{warningCount > 0 ? '▲' : nbsp}</span>
    let message = <span>{warningCount} {warningCount === 1 ? 'Warning' : 'Warnings'}</span>
    return (<div className={warningPanelClasses}>{icon}&nbsp;{message}</div>)
  }

  upPanel(upCount, itemCount) {
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
