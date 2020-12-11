import React from "react"
import styled from "styled-components"
import OverviewGrid from "./OverviewGrid"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewStatusBar from "./OverviewStatusBar"
import OverviewTabBar from "./OverviewTabBar"
import PathBuilder from "./PathBuilder"
import { Color } from "./style-helpers"

type OverviewPaneProps = {
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewPaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  min-height: 100vh;
  background-color: ${Color.grayDark};
`

export default function OverviewPane(props: OverviewPaneProps) {
  return (
    <OverviewPaneRoot>
      <OverviewTabBar pathBuilder={props.pathBuilder} />
      <OverviewResourceBar {...props} />
      <OverviewGrid {...props} />
      <OverviewStatusBar build={props.view.runningTiltBuild} />
    </OverviewPaneRoot>
  )
}
