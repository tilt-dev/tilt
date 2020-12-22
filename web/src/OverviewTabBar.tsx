import React, { useEffect, useState } from "react"
import { matchPath } from "react-router"
import { Link, useHistory } from "react-router-dom"
import styled from "styled-components"
import { useLocalStorageContext } from "./LocalStorage"
import { Color } from "./style-helpers"

type OverviewTabBarProps = {
  tabsForTesting?: string[]
}

let OverviewTabBarRoot = styled.div`
  display: flex;
  width: 100%;
  height: 68px;
  background-color: ${Color.gray};
  border-bottom: 1px solid ${Color.grayLight};
`

export let Tab = styled(Link)`
  display: flex;
  border: 1px solid ${Color.grayLight};
  border-radius: 4px 4px 0px 0px;
  margin: 12px;
  flex-grow: 0;
  padding: 8px;
  text-decoration: none;
`

export default function OverviewTabBar(props: OverviewTabBarProps) {
  let lsc = useLocalStorageContext()
  let history = useHistory()
  let matchTab = matchPath(String(history.location.pathname), {
    path: "/r/:name/overview",
  })

  // The tab that's currently selected.
  // Inferred from the current browser location.
  // If empty, that means we're on a non-tab page, like the overview page.
  let matchParams: any = matchTab?.params
  let selectedTab = matchParams?.name || ""

  // The list of tabs open. A tab name should never appear twice in the list.
  const [tabs, setTabs] = useState<Array<string>>(
    () => props.tabsForTesting ?? lsc.get<Array<string>>("tabs") ?? []
  )

  useEffect(() => {
    lsc.set("tabs", tabs)
  }, [tabs, lsc])

  // Ensures the tab is in the tab list.
  function ensureOpenTab(name: string) {
    if (!name) {
      return
    }

    setTabs((prevState) => {
      if (prevState.includes(name)) {
        return prevState
      }

      return [name].concat(prevState)
    })
  }

  useEffect(() => {
    ensureOpenTab(selectedTab)
  }, [tabs, selectedTab])

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

  let onClose = (e: any, name: string) => {
    e.stopPropagation()
    e.preventDefault()
    closeTab(name)
  }

  let tabEls = tabs.map((name) => {
    let href = `/r/${name}/overview`
    let text = name
    if (selectedTab === name) {
      text += " (current)"
    }
    return (
      <Tab key={name} to={href}>
        <div>{text}</div>
        <button onClick={(e) => onClose(e, name)}>close</button>
      </Tab>
    )
  })
  tabEls.unshift(
    <Tab key="logo" to={"/overview"}>
      Logo
    </Tab>
  )
  return <OverviewTabBarRoot>{tabEls}</OverviewTabBarRoot>
}
