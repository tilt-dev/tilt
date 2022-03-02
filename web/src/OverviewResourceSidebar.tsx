import React, { useMemo } from "react"
import styled from "styled-components"
import { buttonsByComponent } from "./ApiButton"
import { useLogAlertIndex } from "./LogStore"
import { usePathBuilder } from "./PathBuilder"
import { useResourceListOptions } from "./ResourceListOptionsContext"
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
  let buttons = useMemo(
    () => buttonsByComponent(props.view.uiButtons),
    [props.view.uiButtons]
  )
  let items = resources.map((res) => {
    let stopBuildButton = buttons.get(res.metadata?.name!)?.stopBuild
    return new SidebarItem(res, logAlertIndex, stopBuildButton)
  })
  let selected = props.name
  if (props.name === ResourceName.all) {
    selected = ""
  }
  const { options } = useResourceListOptions()

  return (
    <OverviewResourceSidebarRoot>
      <SidebarResources
        items={items}
        selected={selected}
        resourceView={ResourceView.OverviewDetail}
        pathBuilder={pathBuilder}
        resourceListOptions={options}
      />
    </OverviewResourceSidebarRoot>
  )
}
