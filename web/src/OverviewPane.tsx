import React from "react"
import styled from "styled-components"
import { ReactComponent as GridDividerAllSvg } from "./assets/svg/grid-divider-all.svg"
import { ReactComponent as GridDividerPinSvg } from "./assets/svg/grid-divider-pin.svg"
import { ReactComponent as GridDividerTestSvg } from "./assets/svg/grid-divider-test.svg"
import OverviewGrid from "./OverviewGrid"
import { OverviewItem } from "./OverviewItemView"
import OverviewResourceBar from "./OverviewResourceBar"
import OverviewTabBar from "./OverviewTabBar"
import { useSidebarPin } from "./SidebarPin"
import { Color, Font } from "./style-helpers"

type OverviewPaneProps = {
  view: Proto.webviewView
}

let OverviewPaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
`

let ServicesDividerRoot = styled.div`
  display: flex;
  width: 100%;
  align-items: center;
`

let ServicesLabel = styled.div`
  font: ${Font.sansSerif};
  color: ${Color.blue};
  margin: 12px;
`

let ServicesContainer = styled.div`
  flex-grow: 1;
  flex-shrink: 1;
  overflow: auto;
`

function PinnedServicesDivider() {
  return (
    <ServicesDividerRoot>
      <GridDividerPinSvg style={{ marginLeft: "28px" }} />
      <ServicesLabel>Pinned Resources</ServicesLabel>
    </ServicesDividerRoot>
  )
}

function AllResourcesDivider() {
  return (
    <ServicesDividerRoot>
      <GridDividerAllSvg style={{ marginLeft: "28px" }} />
      <ServicesLabel>All Resources</ServicesLabel>
    </ServicesDividerRoot>
  )
}

function TestsDivider() {
  return (
    <ServicesDividerRoot>
      <GridDividerTestSvg style={{ marginLeft: "28px" }} />
      <ServicesLabel>Tests</ServicesLabel>
    </ServicesDividerRoot>
  )
}

export default function OverviewPane(props: OverviewPaneProps) {
  let pinContext = useSidebarPin()
  let resources = props.view.resources || []
  let allItems = resources.map((res) => new OverviewItem(res))
  let allResources = allItems.filter(
    (item) =>
      // NOTE(maia): this is gross naming, but until we have better nouns:
      //  "all resources" = everything but tests
      // (This is bad because in the backend, tests are also "resources", but  ¯\_(ツ)_/¯ )
      !item.isTest
  )
  let pinnedItems = allItems.filter((item) =>
    pinContext.pinnedResources.includes(item.name)
  )

  let pinnedDivider = pinnedItems.length ? <PinnedServicesDivider /> : null
  let pinnedGrid = pinnedItems.length ? (
    <OverviewGrid items={pinnedItems} />
  ) : null
  let testItems = allItems.filter((item) => item.isTest)
  let testDivider = testItems.length ? <TestsDivider /> : null
  let testGrid = testItems.length ? <OverviewGrid items={testItems} /> : null
  return (
    <OverviewPaneRoot>
      <OverviewTabBar selectedTab={""} />
      <OverviewResourceBar view={props.view} />
      <ServicesContainer>
        {pinnedDivider}
        {pinnedGrid}
        <AllResourcesDivider />
        <OverviewGrid items={allResources} />
        {testDivider}
        {testGrid}
      </ServicesContainer>
    </OverviewPaneRoot>
  )
}
