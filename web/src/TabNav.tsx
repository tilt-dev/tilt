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
export type TabNav = {
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
// a) Click on any resource from left sidebar to change logs on right side accordingly
// b) Double click on any resource to open that as a new tab on the immediate right
//   of current tab
// c) (OR, if the resource is already open in a tab, then view toggles to that open tab)
//
// 6. When you open resource in new tab (from *overview grid page*)
// a) Click on a resource card to open preview of that resource inline
// b) Click on "Show details" within card preview, to open that resrouce in the tab view
// c) This new tab opens on the absolute right of all other tabs.
export function OverviewNavProvider(
  props: React.PropsWithChildren<{
    tabsForTesting?: string[]
  }>
) {
  let { children } = props
  let lsc = useLocalStorageContext()
  let history = useHistory()
  let pb = usePathBuilder()

  // The list of tabs open. A tab name should never appear twice in the list.
  const [tabState, setTabState] = useState<{
    tabs: string[]
    selectedTab: string
  }>(() => {
    let tabs = props.tabsForTesting ?? lsc.get<Array<string>>("tabs") ?? []
    return { tabs, selectedTab: "" }
  })
  let tabs = tabState.tabs
  let selectedTab = tabState.selectedTab

  useEffect(() => {
    lsc.set("tabs", tabs)
  }, [tabs, lsc])

  // Ensures the tab is in the tab list.
  function ensureSelectedTab(name: string) {
    if (!name) {
      return
    }

    if (tabs.includes(name) && name == selectedTab) {
      return
    }

    setTabState({
      tabs: tabs.includes(name) ? tabs : tabs.concat([name]),
      selectedTab: name,
    })
  }

  // Deletes the resource in the tab list.
  // If we're deleting the current tab, navigate to the next reasonable tab.
  function closeTab(name: string) {
    let newTabs = tabs.filter((t) => t !== name)
    if (name !== selectedTab) {
      setTabState({ tabs: newTabs, selectedTab: name })
      return
    }

    let index = tabs.indexOf(name)
    let newSelectedTab = ""
    if (index + 1 < tabs.length) {
      newSelectedTab = tabs[index + 1]
    } else if (index - 1 >= 0) {
      newSelectedTab = tabs[index - 1]
    }

    let newUrl = pb.path(`/overview`)
    if (newSelectedTab) {
      newUrl = pb.path(`/r/${newSelectedTab}/overview`)
    }

    // Ideally, we'd use a reducer to set tab state, but we
    // would need to synchronize it with the history state changes.
    // We can revisit this if we see weird behavior.
    setTabState({ tabs: newTabs, selectedTab: newSelectedTab })
    history.push(newUrl)
  }

  let nav = (name: string, openNew: boolean) => {
    name = name || ResourceName.all

    let url = pb.path(`/r/${name}/overview`)
    let tabs = tabState.tabs
    let newTabs
    let selectedIndex = tabs.indexOf(selectedTab)
    let includes = tabs.includes(name)
    if (openNew && includes) {
      // If we're opening a new tab, but the tab already exists, just toggle that tab (case 5c above)
      newTabs = tabs
    } else if (selectedIndex !== -1) {
      // We're navigating from an existing tab. Replace the current tab (on
      // single-click) or open a new tab to the right of the current tab (on double click).
      // (case 1, 2, 5a, 5b above)
      let start = tabs
        .slice(0, openNew ? selectedIndex + 1 : selectedIndex)
        .filter((tab) => tab !== name)
      let end = tabs.slice(selectedIndex + 1).filter((tab) => tab !== name)
      newTabs = start.concat([name]).concat(end)
    } else {
      // Append to absolute right of the tab list if not included.
      // (case 6 above)
      newTabs = includes ? tabs : tabs.concat([name])
    }

    setTabState({ tabs: newTabs, selectedTab: name })
    history.push(url)
  }

  let clickResource = (name: string) => nav(name, false)

  let doubleClickResource = (name: string) => nav(name, true)

  let tabNav = {
    tabs,
    selectedTab,
    ensureSelectedTab,
    closeTab,
    clickResource,
    doubleClickResource,
  }
  return (
    <tabNavContext.Provider value={tabNav}>{children}</tabNavContext.Provider>
  )
}
