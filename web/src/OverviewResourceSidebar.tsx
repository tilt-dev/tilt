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
import ChevronRightIcon from "@material-ui/icons/ChevronRight"
import ChevronLeftIcon from "@material-ui/icons/ChevronLeft"
import { InstrumentedButton } from "./instrumentedComponents"
import { useSidebarContext } from "./SidebarContext"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { Tooltip } from "@material-ui/core"
import { mixinResetButtonStyle } from "./style-helpers"
import { OverviewSidebarToggle } from "./OverviewSidebarToggle"

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
  min-width: ${Width.sidebarMinimum}px;

  &.is-open {
    min-width: ${Width.sidebarDefault}px;
  }
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
  const { isSidebarOpen } = useSidebarContext()

  return (
    <OverviewResourceSidebarRoot className={isSidebarOpen ? "is-open" : ""}>
      {
        /* If the sidebar is toggled, only show the toggle button */
        isSidebarOpen ? (
          <SidebarResources
            items={items}
            selected={selected}
            resourceView={ResourceView.OverviewDetail}
            pathBuilder={pathBuilder}
            resourceListOptions={options}
          />
        ) : (
          <OverviewSidebarToggle />
        )
      }
    </OverviewResourceSidebarRoot>
  )
}
