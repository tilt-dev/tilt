import React from "react"
import styled from "styled-components"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"
import SidebarItem from "./SidebarItem"
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
import { ResourceStatus } from "./types"

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

let TestData = styled.section`
  position: fixed;
  // top: 0;
  // right: 0;
  bottom: ${Height.statusbar}px;
  width: ${Width.sidebar}px;
  background-color: ${Color.blue};
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

function GetTestAggregateData(items: SidebarItem[]) {
  let numTests = 0
  let numGreenTests = 0
  let numRedTests = 0
  // TODO: average duration (just for the most recent run? over all runs?)

  for (let i = 0; i < items.length; i++) {
    let item = items[i]
  
    if (!item.isTest) {
      continue
    }

    numTests++

    // Do we count CURRENT status towards red/green? (i.e. a running/pending test
    // isn't included in this count?) Or the last run, whatever that was (i.e. a
    // test that was just green and is now running is counted as green)? Someone
    // else's problem #yolo
    if (item.buildStatus == ResourceStatus.Healthy) {
      numGreenTests++
    } else if (item.buildStatus == ResourceStatus.Unhealthy) {
      numRedTests++
    }
  }
  return (
    <TestData>
      <div>Number of tests: {numTests}</div>
      <div>Number of green tests: {numGreenTests}</div>
      <div>Number of red tests: {numRedTests}</div>
    </TestData>
  )
}
//   numGreenTests = 0
//   avgTestDur = 0
//   for item in items:
//     if item.isTest:
//       // get data off it and store in our aggregator
//
//   return <div>
//     Number Tests: ${numTests}
//   </div>
// }

function Sidebar(props: SidebarProps) {
  // SHITTY HACK TO GET SIDEBAR ITEMS OUT OF SIDEBAR PROPS
  // WHY DON'T WE PASS THEM AS THEIR OWN PROP? I DON'T KNOW! >:(
  let items = []
  if (props.children.length == 2) {
    items = props.children[1].props.items
  }

  let testData = GetTestAggregateData(items)

  return (
    <SidebarRoot className={`Sidebar ${props.isClosed ? "isClosed" : ""}`}>
      {props.children}
      {testData}
      <SidebarToggle className="Sidebar-toggle" onClick={props.toggleSidebar}>
        <ChevronSvg /> Collapse
      </SidebarToggle>
    </SidebarRoot>
  )
}

export default Sidebar
