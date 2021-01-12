import React from "react"
import styled from "styled-components"
import NotFound from "./NotFound"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewResourceDetails from "./OverviewResourceDetails"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import OverviewTabBar from "./OverviewTabBar"
import { Color } from "./style-helpers"
import { ResourceName } from "./types"

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
  flex: 1 1 auto;
  overflow: hidden;
`

export default function OverviewResourcePane(props: OverviewResourcePaneProps) {
  let resources = props.view?.resources || []
  let name = props.name
  let r: Proto.webviewResource | undefined
  if (name !== "" && name != ResourceName.all) {
    r = resources.find((r) => r.name === name)
    if (!r) {
      return <NotFound location={{ pathname: `/r/${name}/overview` }} />
    }
  }

  return (
    <OverviewResourcePaneRoot>
      <OverviewTabBar />
      <OverviewResourceBar {...props} />
      <Main>
        <OverviewResourceSidebar {...props} />
        <OverviewResourceDetails resource={r} />
      </Main>
    </OverviewResourcePaneRoot>
  )
}
