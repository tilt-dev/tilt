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
import MenuOutlinedIcon from "@material-ui/icons/MenuOutlined"
import MenuOpenOutlinedIcon from "@material-ui/icons/MenuOpenOutlined"
import IconButton from "@material-ui/core/IconButton"
import { useSidebarContext } from "./SidebarContext"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { Tooltip } from "@material-ui/core"

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

let SidebarToggleRoot = styled.div`
  display: flex;
  justify-content: flex-start;
`

const menuIconMixin = `
  display: flex;
  transition: fill ${AnimDuration.default} ease;
  height: 100%;
  fill: ${Color.gray70};
`

const MenuOpenIcon = styled(MenuOutlinedIcon)`
  ${menuIconMixin}
`

const MenuClosedIcon = styled(MenuOpenOutlinedIcon)`
  ${menuIconMixin}
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
  const { isSidebarOpen, setSidebarOpen, setSidebarClosed } =
    useSidebarContext()

  return (
    <OverviewResourceSidebarRoot>
      <SidebarToggleRoot>
        <Tooltip
          title={isSidebarOpen ? "Collapse" : "Expand"}
          placement={isSidebarOpen ? "right" : "bottom"}
        >
          <IconButton
            style={{
              paddingTop: 0,
              paddingBottom: 0,
            }}
            aria-label="Open or close the sidebar"
            onClick={() =>
              isSidebarOpen ? setSidebarClosed() : setSidebarOpen()
            }
          >
            {isSidebarOpen ? <MenuOpenIcon /> : <MenuClosedIcon />}
          </IconButton>
        </Tooltip>
      </SidebarToggleRoot>
      {isSidebarOpen && (
        <SidebarResources
          items={items}
          selected={selected}
          resourceView={ResourceView.OverviewDetail}
          pathBuilder={pathBuilder}
          resourceListOptions={options}
        />
      )}
    </OverviewResourceSidebarRoot>
  )
}
