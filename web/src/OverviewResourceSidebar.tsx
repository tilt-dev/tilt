import React from "react"
import styled from "styled-components"
import OverviewTabBar from "./OverviewTabBar"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import { ResourceView } from "./types"

type OverviewResourceSidebarProps = {
  name: string
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewResourceSidebarRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 380px;
  height: 100%;
`

export default function OverviewResourceSidebar(
  props: OverviewResourceSidebarProps
) {
  let resources = props.view.resources || []
  let items = resources.map((res) => new SidebarItem(res))

  return (
    <OverviewResourceSidebarRoot>
      <OverviewTabBar {...props} logoOnly={true} />
      <div>16 Resources</div>
      <div>3 errors | 0 warnings</div>
      <SidebarResources
        items={items}
        selected={props.name}
        resourceView={ResourceView.Overview}
        pathBuilder={props.pathBuilder}
      />
    </OverviewResourceSidebarRoot>
  )
}
