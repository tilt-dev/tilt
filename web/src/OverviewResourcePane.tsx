import React, { useEffect } from "react"
import styled from "styled-components"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewResourceDetails from "./OverviewResourceDetails"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import OverviewTabBar from "./OverviewTabBar"
import { Color } from "./style-helpers"
import { useTabNav } from "./TabNav"
import { ResourceName } from "./types"

type OverviewResourcePaneProps = {
  view: Proto.webviewView
}

let OverviewResourcePaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
  max-height: 100%;
`

let Main = styled.div`
  display: flex;
  width: 100%;
  // In Safari, flex-basis "auto" squishes OverviewTabBar + OverviewResourceBar
  flex: 1 1 100%;
  overflow: hidden;
`

export default function OverviewResourcePane(props: OverviewResourcePaneProps) {
  let nav = useTabNav()
  let resources = props.view?.resources || []
  let name = nav.invalidTab || nav.selectedTab || ""
  let r: Proto.webviewResource | undefined
  let all = name === "" || name === ResourceName.all
  if (!all) {
    r = resources.find((r) => r.name === name)
  }
  let selectedTab = ""
  if (all) {
    selectedTab = ResourceName.all
  } else if (r?.name) {
    selectedTab = r.name
  }

  // Hide the HTML element scrollbars, since this pane does all scrolling internally.
  // TODO(nick): Remove this when the old UI is deleted.
  useEffect(() => {
    document.documentElement.style.overflow = "hidden"
  })

  return (
    <OverviewResourcePaneRoot>
      <OverviewTabBar selectedTab={selectedTab} />
      <OverviewResourceBar {...props} />
      <Main>
        <OverviewResourceSidebar {...props} name={name} />
        <OverviewResourceDetails resource={r} name={name} />
      </Main>
    </OverviewResourcePaneRoot>
  )
}
