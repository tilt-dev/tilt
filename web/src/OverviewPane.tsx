import React from "react"
import styled from "styled-components"
import { ReactComponent as AllServicesSvg } from "./assets/svg/all-services.svg"
import OverviewGrid from "./OverviewGrid"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewStatusBar from "./OverviewStatusBar"
import OverviewTabBar from "./OverviewTabBar"
import PathBuilder from "./PathBuilder"
import { Color, Font, SizeUnit } from "./style-helpers"

type OverviewPaneProps = {
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewPaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
`

let AllServicesDividerRoot = styled.div`
  display: flex;
  width: 100%;
  align-items: center;
`

let AllServicesDashLeft = styled.div`
  width: ${SizeUnit(0.5)};
  height: 0;
  border-bottom: 1px dashed ${Color.grayLight};
`

let AllServicesDashRight = styled.div`
  flex-grow: 1;
  height: 0;
  border-bottom: 1px dashed ${Color.grayLight};
`

let AllServicesLabel = styled.div`
  font: ${Font.sansSerif};
  color: ${Color.blue};
  margin: 12px;
`

function AllServicesDivider() {
  return (
    <AllServicesDividerRoot>
      <AllServicesDashLeft />
      <AllServicesSvg style={{ marginLeft: "12px" }} />
      <AllServicesLabel>All services</AllServicesLabel>
      <AllServicesDashRight />
    </AllServicesDividerRoot>
  )
}

export default function OverviewPane(props: OverviewPaneProps) {
  return (
    <OverviewPaneRoot>
      <OverviewTabBar pathBuilder={props.pathBuilder} />
      <OverviewResourceBar {...props} />
      <AllServicesDivider />
      <OverviewGrid {...props} />
      <OverviewStatusBar build={props.view.runningTiltBuild} />
    </OverviewPaneRoot>
  )
}
