import React, { Component } from "react"
import { History } from "history"
import SidebarItem from "./SidebarItem"
import PathBuilder from "./PathBuilder"

type Props = {
  items: SidebarItem[]
  selected: string
  pathBuilder: PathBuilder
  history: History
}

/**
 * Sets up keyboard shortcuts that depend on the state of the sidebar.
 */
class SidebarKeyboardShortcuts extends Component<Props> {
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
    if (e.shiftKey || e.metaKey || e.altKey || e.ctrlKey || e.isComposing) {
      return
    }

    let items = this.props.items
    let selected = this.props.selected || ""
    let pathBuilder = this.props.pathBuilder
    let history = this.props.history
    switch (e.key) {
      case "j":
      case "k":
        // An array of sidebar items, plus one at the beginning for 'All'
        let names = [""].concat(items.map(item => item.name))
        let index = names.indexOf(selected)
        let dir = e.key === "j" ? 1 : -1
        let targetIndex = index + dir
        if (targetIndex < 0 || targetIndex >= names.length) {
          return
        }

        let name = names[targetIndex]
        let path = name ? pathBuilder.path(`/r/${name}`) : pathBuilder.path("/")
        history.push(path)
        e.preventDefault()
    }
  }

  render() {
    return <span></span>
  }
}

export default SidebarKeyboardShortcuts
