import React, { Component } from "react"
import { isTargetEditable } from "./shortcut"
import SidebarItem from "./SidebarItem"
import { TabNav, useTabNav } from "./TabNav"
import { ResourceName, ResourceView } from "./types"

type Props = {
  items: SidebarItem[]
  selected: string
  tabNav: TabNav
  resourceView: ResourceView
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
    if (isTargetEditable(e)) {
      return
    }

    if (e.shiftKey || e.altKey || e.isComposing) {
      return
    }

    let items = this.props.items
    let selected = this.props.selected || ResourceName.all
    switch (e.key) {
      case "j":
      case "k":
        // An array of sidebar items, plus one at the beginning for 'All'
        let names = [ResourceName.all as string].concat(
          items.map((item) => item.name)
        )
        let index = names.indexOf(selected)
        let dir = e.key === "j" ? 1 : -1
        let targetIndex = index + dir
        if (targetIndex < 0 || targetIndex >= names.length) {
          return
        }

        let name = names[targetIndex]
        this.props.tabNav.openResource(name, { newTab: e.metaKey || e.ctrlKey })
        e.preventDefault()
        break

      case "r":
        if (e.metaKey || e.ctrlKey) {
          return
        }
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
  onTrigger: (action: string) => void
  resourceView: ResourceView
}

export default function (props: PublicProps) {
  let tabNav = useTabNav()
  return <SidebarKeyboardShortcuts {...props} tabNav={tabNav} />
}
