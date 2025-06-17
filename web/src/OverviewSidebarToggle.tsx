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

let SidebarToggleRoot = styled.div`
  display: flex;
  justify-content: flex-end;

  &.is-open {
    margin-right: -16px;
  }
`

const ToggleSidebarButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle}
  display: flex;
  align-items: center;

  svg {
    fill: ${Color.gray50};
  }

  &:hover svg {
    fill: ${Color.gray70};
  }
`

export function OverviewSidebarToggle() {
  const { isSidebarOpen, setSidebarClosed, setSidebarOpen } =
    useSidebarContext()
  return (
    <SidebarToggleRoot className={isSidebarOpen ? "is-open" : ""}>
      <Tooltip
        title={isSidebarOpen ? "Collapse sidebar" : "Expand sidebar"}
        placement={isSidebarOpen ? "right" : "bottom"}
      >
        <ToggleSidebarButton
          aria-label={isSidebarOpen ? "Collapse sidebar" : "Expand sidebar"}
          onClick={() =>
            isSidebarOpen ? setSidebarClosed() : setSidebarOpen()
          }
        >
          {isSidebarOpen ? <ChevronLeftIcon /> : <ChevronRightIcon />}
        </ToggleSidebarButton>
      </Tooltip>
    </SidebarToggleRoot>
  )
}
