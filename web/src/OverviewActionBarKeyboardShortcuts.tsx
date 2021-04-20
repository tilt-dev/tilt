import React, { Component } from "react"
import { incr } from "./analytics"
import { clearLogs } from "./ClearLogs"
import LogStore from "./LogStore"
import { isTargetEditable } from "./shortcut"
import { ResourceName } from "./types"

type Link = Proto.webviewLink

type Props = {
  logStore: LogStore
  resourceName: string
  endpoints?: Link[]
  openEndpointUrl: (url: string) => void
}

const keyCodeOne = 49
const keyCodeNine = 57

/**
 * Sets up keyboard shortcuts that depend on the state of the current resource.
 */
class OverviewActionBarKeyboardShortcuts extends Component<Props> {
  constructor(props: Props) {
    super(props)
    this.onKeydown = this.onKeydown.bind(this)
  }

  componentDidMount() {
    document.body.addEventListener("keydown", this.onKeydown)
  }

  componentWillUnmount() {
    document.body.removeEventListener("keydown", this.onKeydown)
  }

  onKeydown(e: KeyboardEvent) {
    if (isTargetEditable(e)) {
      return
    }

    if (e.altKey || e.isComposing) {
      return
    }

    if (e.ctrlKey || e.metaKey) {
      if (e.key === "Backspace" && !e.shiftKey) {
        const all = this.props.resourceName === ResourceName.all
        incr("ui.web.clearLogs", { action: "shortcut", all: all.toString() })
        clearLogs(this.props.logStore, this.props.resourceName)
        e.preventDefault()
        return
      }

      return
    }

    if (e.keyCode >= keyCodeOne && e.keyCode <= keyCodeNine && e.shiftKey) {
      let endpointIndex = e.keyCode - keyCodeOne
      let endpoint = this.props.endpoints && this.props.endpoints[endpointIndex]
      if (!endpoint || !endpoint.url) {
        return
      }

      incr("ui.web.endpoint", { action: "shortcut" })
      this.props.openEndpointUrl(endpoint.url)
      e.preventDefault()
      return
    }
  }

  render() {
    return <></>
  }
}

export default OverviewActionBarKeyboardShortcuts
