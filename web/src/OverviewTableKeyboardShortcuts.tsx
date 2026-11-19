import React, { Component } from "react"
import { RowValues } from "./OverviewTableColumns"
import { useResourceNav } from "./ResourceNav"
import {
  ResourceSelectionContext,
  useResourceSelection,
} from "./ResourceSelectionContext"
import { isTargetEditable } from "./shortcut"
import { useStarredResources } from "./StarredResourcesContext"
import { startBuild } from "./trigger"
import { ResourceName } from "./types"

type Props = {
  rows: RowValues[]
  focused: string
  setFocused: (focused: string) => void
  selection: ResourceSelectionContext
  // What the details panel is showing, if it's open.
  openName: string
  openResource: (name: string) => void
  // The combined logs ("All Resources", "Starred Resources") listed above
  // the table. With the panel open, the keys move through these too.
  builtinNames: string[]
}

// After a click in the log pane, arrow keys scroll the logs
// instead of moving through the table.
const LOG_PANE_SELECTOR = '[aria-label="Log pane"]'

/**
 * Sets up keyboard shortcuts that depend on the state of the sidebar.
 */
class Shortcuts extends Component<Props> {
  constructor(props: Props) {
    super(props)
    this.onKeydown = this.onKeydown.bind(this)
    this.onMousedown = this.onMousedown.bind(this)
  }

  lastClickInLogs = false

  componentDidMount() {
    document.body.addEventListener("keydown", this.onKeydown)
    document.body.addEventListener("mousedown", this.onMousedown)
  }

  componentWillUnmount() {
    document.body.removeEventListener("keydown", this.onKeydown)
    document.body.removeEventListener("mousedown", this.onMousedown)
  }

  onMousedown(e: MouseEvent) {
    this.lastClickInLogs =
      e.target instanceof Element && !!e.target.closest(LOG_PANE_SELECTOR)
  }

  onKeydown(e: KeyboardEvent) {
    if (isTargetEditable(e) || e.shiftKey || e.altKey || e.isComposing) {
      return
    }

    let items = this.props.rows
    let openName = this.props.openName
    // While a details panel is open, the keys move from the open resource.
    let focused = openName || this.props.focused
    let dir = 0
    switch (e.key) {
      case "Down":
      case "ArrowDown":
      case "Up":
      case "ArrowUp":
        if (this.lastClickInLogs) {
          return
        }
        dir = e.key.endsWith("Down") ? 1 : -1
        break

      case "j":
        dir = 1
        break

      case "k":
        dir = -1
        break

      case "Enter":
        // Open the highlighted row, unless Enter is pressing a button
        if (
          !focused ||
          openName ||
          (e.target instanceof Element && e.target.closest("button, a"))
        ) {
          return
        }
        this.props.openResource(focused)
        e.preventDefault()
        return

      case "r":
        // Re-run the open resource
        if (e.metaKey || e.ctrlKey || !items.some((i) => i.name == openName)) {
          return
        }
        startBuild(openName)
        e.preventDefault()
        return

      case "x":
        // The selection checkboxes are hidden while the panel is open
        if (e.metaKey || e.ctrlKey || openName) {
          return
        }
        let item = items.find((item) => item.name == focused)
        if (item && item.selectable) {
          let selection = this.props.selection
          if (selection.isSelected(item.name)) {
            this.props.selection.deselect(item.name)
          } else {
            this.props.selection.select(item.name)
          }
          e.preventDefault()
        }
        break
    }

    if (dir != 0) {
      // Select up and down the list.
      let names = items.map((item) => item.name)
      if (openName) {
        names = this.props.builtinNames.concat(names)
      }
      let index = names.indexOf(focused)
      let targetIndex = 0
      if (index != -1) {
        targetIndex = index + dir
      }

      if (targetIndex < 0 || targetIndex >= names.length) {
        return
      }

      let name = names[targetIndex]
      this.props.setFocused(name)
      if (openName) {
        this.props.openResource(name)
      }
      e.preventDefault()
      return
    }
  }

  render() {
    return <span></span>
  }
}

type PublicProps = {
  rows: RowValues[]
  focused: string
  setFocused: (focused: string) => void
}

export function OverviewTableKeyboardShortcuts(props: PublicProps) {
  let selection = useResourceSelection()
  let nav = useResourceNav()
  let { starredResources } = useStarredResources()
  let builtinNames: string[] = [ResourceName.all]
  if (starredResources.length) {
    builtinNames.push(ResourceName.starred)
  }
  // Only what's listed counts (not, say, a link to a missing resource).
  let current = nav.invalidResource || nav.selectedResource
  let openName =
    builtinNames.includes(current) || props.rows.some((r) => r.name === current)
      ? current
      : ""
  return (
    <Shortcuts
      {...props}
      selection={selection}
      openName={openName}
      openResource={nav.openResource}
      builtinNames={builtinNames}
    />
  )
}
