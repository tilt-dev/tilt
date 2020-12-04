import React, { Component } from "react"
import { History } from "history"
import SidebarItem from "./SidebarItem"
import PathBuilder from "./PathBuilder"
import { useHistory } from "react-router"

type Props = {
  items: SidebarItem[]
  selected: string
  pathBuilder: PathBuilder
  history: History
  onTrigger: (action: string) => void
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
        let names = [""].concat(items.map((item) => item.name))
        let index = names.indexOf(selected)
        let dir = e.key === "j" ? 1 : -1
        let targetIndex = index + dir
        if (targetIndex < 0 || targetIndex >= names.length) {
          return
        }

        let name = names[targetIndex]
        let path = name ? pathBuilder.path(`/r/${name}`) : pathBuilder.path("/")
        history.push(path, { action: "shortcut" })
        e.preventDefault()
        break

      case "r":
        this.props.onTrigger("shortcut")
        e.preventDefault()
        break
    }
  }

  render() {
    return <span></span>
  }
}

type PublicProps = {
  items: SidebarItem[]
  selected: string
  pathBuilder: PathBuilder
  onTrigger: (action: string) => void
}

export default function (props: PublicProps) {
  let history = useHistory()
  return <SidebarKeyboardShortcuts {...props} history={history} />
}
