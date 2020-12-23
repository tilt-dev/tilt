import React from "react"
import styled from "styled-components"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import {
  AnimDuration,
  Color,
  Font,
  FontSize,
  Height,
  SizeUnit,
  Width,
  ZIndex,
} from "./style-helpers"

let SidebarToggle = styled.button`
  background-color: ${Color.grayDarkest};
  border: 0 none;
  color: inherit;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.small};
  line-height: 1;
  text-transform: uppercase;
  display: flex;
  align-items: center;
  cursor: pointer;
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  transition-property: color, background-color;
  transition-duration: ${AnimDuration.default};
  transition-timing-function: ease;

  &:hover {
    color: ${Color.blueLight};
  }

  > svg {
    fill: ${Color.white};
    width: ${Width.sidebarCollapsed}px;
    transition: transform ${AnimDuration.default} ease-in,
      fill ${AnimDuration.default} ease;
  }

  &:hover > svg {
    fill: ${Color.blueLight};
  }
`

let SidebarRoot = styled.section`
  position: fixed;
  top: 0;
  right: 0;
  bottom: ${Height.statusbar}px;
  width: ${Width.sidebar}px;
  background-color: ${Color.grayDarkest};
  box-sizing: border-box;
  overflow-y: auto;
  transform: translateX(0%);
  transition: transform ease ${AnimDuration.default};
  font-size: ${FontSize.default};
  display: flex;
  flex-direction: column;
  z-index: ${ZIndex.Sidebar};

  &.isClosed {
    transform: translateX(calc(100% - ${Width.sidebarCollapsed}px));
  }

  &.isClosed ${SidebarToggle} > svg {
    transform: rotate(180deg);
  }
`

type SidebarProps = {
  children: any
  isClosed: boolean
  toggleSidebar: any
}

function Sidebar(props: SidebarProps) {
  return (
    <SidebarRoot className={`Sidebar ${props.isClosed ? "isClosed" : ""}`}>
      {props.children}
      <SidebarToggle className="Sidebar-toggle" onClick={props.toggleSidebar}>
        <ChevronSvg /> Collapse
      </SidebarToggle>
    </SidebarRoot>
  )
}

export default Sidebar
