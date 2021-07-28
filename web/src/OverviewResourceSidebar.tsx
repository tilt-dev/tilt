import React from "react"
import styled from "styled-components"
import { useLogAlertIndex } from "./LogStore"
import { usePathBuilder } from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import { Width } from "./style-helpers"
import { ResourceName, ResourceView } from "./types"

type OverviewResourceSidebarProps = {
  name: string
  view: Proto.webviewView
}

let OverviewResourceSidebarRoot = styled.div`
  display: flex;
  flex-direction: column;
  flex-shrink: 1;
  flex-grow: 1;
  height: 100%;
  min-width: ${Width.sidebarDefault}px;
`

export default function OverviewResourceSidebar(
  props: OverviewResourceSidebarProps
) {
  let logAlertIndex = useLogAlertIndex()
  let pathBuilder = usePathBuilder()
  let resources = props.view.uiResources || []
  let items = resources.map((res) => new SidebarItem(res, logAlertIndex))
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
