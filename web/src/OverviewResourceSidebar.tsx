import React from "react"
import styled from "styled-components"
import { usePathBuilder } from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import { ResourceName, ResourceView } from "./types"

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
