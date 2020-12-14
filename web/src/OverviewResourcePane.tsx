import React from "react"
import styled from "styled-components"
import NotFound from "./NotFound"
import OverviewResourceDetails from "./OverviewResourceDetails"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import OverviewStatusBar from "./OverviewStatusBar"
import PathBuilder from "./PathBuilder"
import { Color } from "./style-helpers"

type OverviewResourcePaneProps = {
  name: string
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewResourcePaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
`

let Main = styled.div`
  display: flex;
  width: 100%;
  flex-grow: 1;
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
      <Main>
        <OverviewResourceSidebar {...props} />
        <OverviewResourceDetails {...props} />
      </Main>
      <OverviewStatusBar build={props.view.runningTiltBuild} />
    </OverviewResourcePaneRoot>
  )
}
