import React from "react"
import styled from "styled-components"
import { usePathBuilder } from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import { ResourceName, ResourceView } from "./types"
import { Width } from "./style-helpers"

type OverviewResourceSidebarProps = {
  name: string
  view: Proto.webviewView
}

let OverviewResourceSidebarRoot = styled.div`
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  flex-grow: 1;
  height: 100%;
  min-width: ${Width.sidebarDefault}px;
`

export default function OverviewResourceSidebar(
  props: OverviewResourceSidebarProps
) {
  let pathBuilder = usePathBuilder()
  let resources = props.view.uiResources || []
  let items = resources.map((res) => new SidebarItem(res))
  let selected = props.name
  if (props.name === ResourceName.all) {
    selected = ""
  }

  return (
    <OverviewResourceSidebarRoot>
      <SidebarResources
        items={items}
        selected={selected}
        resourceView={ResourceView.OverviewDetail}
        pathBuilder={pathBuilder}
      />
    </OverviewResourceSidebarRoot>
  )
}
