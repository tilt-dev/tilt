import React, { useContext, useEffect } from "react"
import { matchPath, useHistory } from "react-router-dom"
import { usePersistentState } from "./LocalStorage"
import { usePathBuilder } from "./PathBuilder"
import { ResourceName, ResourceView } from "./types"

// Tav navigation semantics.
//
// Different UI controls on the page can have complex interactions
// with the Tab bar, so we model the TabBar as a React context
// shared by multiple components.
export type TabNav = {
  tabs: string[]

  // The currently selected tab. This is guaranteed to exist in the tab list.
  selectedTab: string

  // Tab provided from the user URL that didn't exist.
  // Different parts of the UI might display this error differently.
  invalidTab: string

  // Behavior when you click on a link to a resource.
  openResource(name: string, options?: { newTab: boolean }): void

  // Behavior when you close a tab.
  closeTab(name: string): void
}

const tabNavContext = React.createContext<TabNav>({
  tabs: [],
  selectedTab: "",
  invalidTab: "",
  openResource: (name: string, options?: { newTab: boolean }) => {},
  closeTab: (name: string) => {},
})

export function useTabNav(): TabNav {
  return useContext(tabNavContext)
}

export let TabNavContextConsumer = tabNavContext.Consumer
export let TabNavContextProvider = tabNavContext.Provider

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
    invalidTab: "",
    openResource: nav,
    closeTab: () => {},
  }

  return (
    <tabNavContext.Provider value={tabNav}>{children}</tabNavContext.Provider>
  )
}

function addAllTabIfEmpty(tabs: string[]): string[] {
  if (!tabs.length) {
    return [ResourceName.all]
  }
  return tabs
}

// New Overview semantics:
//
// Any resource supports two navigation operations: "activate" and
// "activate-new-tab".  The exact input bindings are user-agent OS-specific, but
// without loss of generality, treat them as "click" and "ctrl/command-click"
//
// 1) When you activate or activate-new-tab a resource on the left sidebar that's already open,
//    it changes the current tab to the new resource.
//
// 2) Otherwise, when you select a resource on the left sidebar,
//    a) activate opens it in the current tab,
//    b) activate-new-tab opens it in a new tab on the immediate right of current tab
//       (or at the far-right if you're on the grid)
//
// 3. When you close a tab that is currently selected, the view toggles to the tab on the right
export function OverviewNavProvider(
  props: React.PropsWithChildren<{
    validateTab: (name: string) => boolean
    tabsForTesting?: string[]
  }>
) {
  let { children, validateTab, tabsForTesting } = props
  const [tabs, setTabs] = usePersistentState<string[]>(
    "tabs",
    addAllTabIfEmpty(tabsForTesting || [])
  )
  let history = useHistory()
  let pb = usePathBuilder()
  let selectedTab = ""
  let invalidTab = ""

  let matchResource = matchPath(history.location.pathname, {
    path: pb.path("/r/:name"),
  })
  let candidateTab = (matchResource?.params as any)?.name || ""
  if (candidateTab && validateTab(candidateTab)) {
    selectedTab = candidateTab
  } else {
    invalidTab = candidateTab
  }

  useEffect(() => {
    if (selectedTab && !tabs.includes(selectedTab)) {
      setTabs(tabs.concat([selectedTab]))
    }
  }, [tabs, selectedTab])

  // Deletes the resource in the tab list.
  // If we're deleting the current tab, navigate to the next reasonable tab.
  function closeTab(name: string) {
    let newTabs = tabs.filter((t) => t !== name)
    if (name !== selectedTab) {
      setTabs(addAllTabIfEmpty(newTabs))
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
    setTabs(addAllTabIfEmpty(newTabs))
    history.push(newUrl)
  }

  let openResource = (name: string, options?: { newTab: boolean }) => {
    name = name || ResourceName.all
    let openNew = options?.newTab || false
    let url = pb.path(`/r/${name}/overview`)
    let newTabs
    let selectedIndex = tabs.indexOf(selectedTab)
    let includes = tabs.includes(name)
    if (includes) {
      // If we're opening a new tab, but the tab already exists, just toggle that tab
      // (case 1 above)
      newTabs = tabs
    } else if (selectedIndex !== -1) {
      // We're navigating from an existing tab. Replace the current tab (on
      // single-click) or open a new tab to the right of the current tab (on ctrl-click).
      // (case 2a, 2b above)
      let start = tabs
        .slice(0, openNew ? selectedIndex + 1 : selectedIndex)
        .filter((tab) => tab !== name)
      let end = tabs.slice(selectedIndex + 1).filter((tab) => tab !== name)
      newTabs = start.concat([name]).concat(end)
    } else {
      // Append to absolute right of the tab list if not included.
      // (case 2c above)
      newTabs = includes ? tabs : tabs.concat([name])
    }

    setTabs(newTabs)
    history.push(url)
  }

  let tabNav = {
    tabs,
    invalidTab,
    selectedTab,
    closeTab,
    openResource,
  }
  return (
    <tabNavContext.Provider value={tabNav}>{children}</tabNavContext.Provider>
  )
}
