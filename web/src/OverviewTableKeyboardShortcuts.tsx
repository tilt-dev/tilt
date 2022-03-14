import React, { Component } from "react"
import { RowValues } from "./OverviewTableColumns"
import {
  ResourceSelectionContext,
  useResourceSelection,
} from "./ResourceSelectionContext"
import { isTargetEditable } from "./shortcut"

type Props = {
  rows: RowValues[]
  focused: string
  setFocused: (focused: string) => void
  selection: ResourceSelectionContext
}

/**
 * Sets up keyboard shortcuts that depend on the state of the sidebar.
 */
class Shortcuts extends Component<Props> {
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
    if (isTargetEditable(e) || e.shiftKey || e.altKey || e.isComposing) {
      return
    }

    let items = this.props.rows
    let focused = this.props.focused
    let dir = 0
    switch (e.key) {
      case "Down":
      case "ArrowDown":
      case "j":
        dir = 1
        break

      case "Up":
      case "ArrowUp":
      case "k":
        dir = -1
        break

      case "x":
        if (e.metaKey || e.ctrlKey) {
          return
        }
        let item = items.find((item) => item.name == focused)
        if (item) {
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
      let index = names.indexOf(focused)
      let targetIndex = 0
      if (index != -1) {
        let dir = e.key === "j" ? 1 : -1
        targetIndex = index + dir
      }

      if (targetIndex < 0 || targetIndex >= names.length) {
        return
      }

      let name = names[targetIndex]
      this.props.setFocused(name)
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
  return <Shortcuts {...props} selection={selection} />
}
