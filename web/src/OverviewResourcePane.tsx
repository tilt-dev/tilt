import React from "react"
import styled from "styled-components"
import NotFound from "./NotFound"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewResourceDetails from "./OverviewResourceDetails"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import OverviewStatusBar from "./OverviewStatusBar"
import OverviewTabBar from "./OverviewTabBar"
import { Color } from "./style-helpers"

type OverviewResourcePaneProps = {
  name: string
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
  let resources = props.view?.resources || []
  let name = props.name
  let r = resources.find((r) => r.name === name)
  if (r === undefined) {
    return <NotFound location={{ pathname: `/r/${name}/overview` }} />
  }

  return (
    <OverviewResourcePaneRoot>
      <OverviewTabBar />
      <OverviewResourceBar {...props} />
      <Main>
        <OverviewResourceSidebar {...props} />
        <OverviewResourceDetails {...props} />
      </Main>
      <OverviewStatusBar build={props.view.runningTiltBuild} />
    </OverviewResourcePaneRoot>
  )
}
