import React from "react"
import styled from "styled-components"
import SidebarItem from "./SidebarItem"
import {
  AnimDuration,
  Color,
  FontSize,
  Height,
  Width,
  ZIndex,
} from "./style-helpers"
import { ResourceStatus } from "./types"

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
`

type TestAggregateDataProps = {
  items: SidebarItem[]
}

function TestAggregateData(props: TestAggregateDataProps) {
  let numTests = 0
  let numGreenTests = 0
  let numRedTests = 0
  // TODO: average duration (just for the most recent run? over all runs?)

  for (let i = 0; i < props.items.length; i++) {
    let item = props.items[i]

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

  if (numTests === 0) {
    return null
  }
  return (
      <TestData className="TestAggregateData">
        <div>Number of tests: <span className="numTests">{numTests}</span></div>
        <div>Number of green tests: <span className="numGreenTests">{numGreenTests}</span></div>
        <div>Number of red tests: <span className="numRedTests">{numRedTests}</span></div>
      </TestData>
  )
}

export default TestAggregateData
