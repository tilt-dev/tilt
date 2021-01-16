import React, { useContext, useEffect, useState } from "react"
import { useHistory } from "react-router-dom"
import { useLocalStorageContext } from "./LocalStorage"
import { usePathBuilder } from "./PathBuilder"
import { ResourceName, ResourceView } from "./types"

// Tav navigation semantics.
//
// Different UI controls on the page can have complex interactions
// with the Tab bar, so we model the TabBar as a React context
// shared by multiple components.
type TabNav = {
  tabs: string[]
  selectedTab: string

  // Behavior when you click on a link to a resource.
  clickResource(name: string): void

  // Behavior when you double click on a link to a resource.
  doubleClickResource(name: string): void

  // Behavior when you close a tab.
  closeTab(name: string): void

  // Make sure the given tab is open and selected.
  ensureSelectedTab(name: string): void
}

const tabNavContext = React.createContext<TabNav>({
  tabs: [],
  selectedTab: "",
  clickResource: (name: string) => {},
  doubleClickResource: (name: string) => {},
  ensureSelectedTab: () => {},
  closeTab: (name: string) => {},
})

export function useTabNav(): TabNav {
  return useContext(tabNavContext)
}

export let TabNavContextConsumer = tabNavContext.Consumer

// In the legacy UI, there are no tabs at all.
// We only need to make sure we're opening the right link.
export function LegacyNavProvider(
  props: React.PropsWithChildren<{ resourceView: ResourceView }>
) {
  let history = useHistory()
  let pb = usePathBuilder()
  let { resourceView, children } = props
  let nav = (name: string) => {
    let all = name === "" || name === ResourceName.all
    if (all) {
      switch (resourceView) {
        case ResourceView.Alerts:
          history.push(pb.path(`/alerts`))
          return
        default:
          history.push(pb.path(`/`))
          return
      }
    }

    switch (props.resourceView) {
      case ResourceView.Alerts:
        history.push(pb.path(`/r/${name}/alerts`))
        return
      case ResourceView.Facets:
        history.push(pb.path(`/r/${name}/facets`))
        return
      default:
        history.push(pb.path(`/r/${name}`))
        return
    }
  }

  let tabNav = {
    tabs: [],
    selectedTab: "",
    clickResource: nav,
    doubleClickResource: nav,
    ensureSelectedTab: () => {},
    closeTab: () => {},
  }

  return (
    <tabNavContext.Provider value={tabNav}>{children}</tabNavContext.Provider>
  )
}

// New Overview semantics:
// TODO(nick): Implement these. We've currently ported this straight over from the old semantics.
//
// 1. When you single click a resource on the left sidebar,
//    it changes the current tab to the new resource.
//
// 2. When you double click a resource on the left sidebar,
//    it opens a new tab, and brings the new tab into focus.
//
// 3. When you close a tab that is currently selected, the view toggles to the tab on the right
//
// 4. When there is only one tab remaining, and you close it, then the overview grid page opens
//
// 5. When you open a resource as a new tab (from *resource detail tab view*)
// - Click on any resource from left sidebar to change logs on right side accordingly
// - Double click on any resource to open that as a new tab on the immediate right
//   of current tab
//   (OR, if the resource is already open in a tab, then view toggles to that open tab)
//
// 6. When you open resource in new tab (from *overview grid page*)
// - Click on a resource card to open preview of that resource inline
// - Click on "Show details" within card preview, to open that resrouce in the tab view
// - This new tab opens on the absolute right of all other tabs.
export function OverviewNavProvider(
  props: React.PropsWithChildren<{
    resourceView: ResourceView
    tabsForTesting?: string[]
  }>
) {
  let { resourceView, children } = props
  let lsc = useLocalStorageContext()
  let history = useHistory()
  let pb = usePathBuilder()

  // The list of tabs open. A tab name should never appear twice in the list.
  const [tabs, setTabs] = useState<Array<string>>(
    () => props.tabsForTesting ?? lsc.get<Array<string>>("tabs") ?? []
  )
  const [selectedTab, setSelectedTab] = useState("")

  useEffect(() => {
    lsc.set("tabs", tabs)
  }, [tabs, lsc])

  // Ensures the tab is in the tab list.
  function ensureSelectedTab(name: string) {
    if (!name) {
      return
    }

    setTabs((prevState) => {
      if (prevState.includes(name)) {
        return prevState
      }

      return [name].concat(prevState)
    })
    setSelectedTab(name)
  }

  // Deletes the resource in the tab list.
  // If we're deleting the current tab, navigate to the next reasonable tab.
  function closeTab(name: string) {
    let newState = (prevState: string[]) => {
      return prevState.filter((t) => t !== name)
    }
    if (name !== selectedTab) {
      setTabs(newState)
      return
    }

    let index = tabs.indexOf(name)
    let desired = `/overview`
    if (index + 1 < tabs.length) {
      desired = `/r/${tabs[index + 1]}/overview`
    } else if (index - 1 >= 0) {
      desired = `/r/${tabs[index - 1]}/overview`
    }

    setTabs(newState)
    history.push(desired)
  }

  let nav = (name: string) => {
    let all = name === "" || name === ResourceName.all
    if (all) {
      history.push(pb.path(`/r/${ResourceName.all}/overview`))
      return
    }

    history.push(pb.path(`/r/${name}/overview`))
  }

  let tabNav = {
    tabs,
    selectedTab,
    ensureSelectedTab,
    closeTab,
    clickResource: nav,
    doubleClickResource: nav,
  }
  return (
    <tabNavContext.Provider value={tabNav}>{children}</tabNavContext.Provider>
  )
}
