import React from "react"
import styled from "styled-components"
import { usePathBuilder } from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import { ResourceView } from "./types"

type OverviewResourceSidebarProps = {
  name: string
  view: Proto.webviewView
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
  let pathBuilder = usePathBuilder()
  let resources = props.view.resources || []
  let items = resources.map((res) => new SidebarItem(res))

  return (
    <OverviewResourceSidebarRoot>
      <div>12/16 up</div>
      <SidebarResources
        items={items}
        selected={props.name}
        resourceView={ResourceView.Overview}
        pathBuilder={pathBuilder}
      />
    </OverviewResourceSidebarRoot>
  )
}
