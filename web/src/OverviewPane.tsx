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

function PinnedResources(items: OverviewItem[]) {
  return items?.length ? (
    <div className={"resources-container pinned"}>
      <ServicesDividerRoot className={"ServicesDivider pinned"}>
        <GridDividerPinSvg style={{ marginLeft: "28px" }} />
        <ServicesLabel>Pinned Resources</ServicesLabel>
      </ServicesDividerRoot>
      <OverviewGrid items={items} />
    </div>
  ) : null
}

function AllResources(items: OverviewItem[]) {
  return items?.length ? (
    <div className={"resources-container all"}>
      <ServicesDividerRoot className={"ServicesDivider all"}>
        <GridDividerAllSvg style={{ marginLeft: "28px" }} />
        <ServicesLabel>All Resources</ServicesLabel>
      </ServicesDividerRoot>
      <OverviewGrid items={items} />
    </div>
  ) : null
}

function TestResources(items: OverviewItem[]) {
  return items?.length ? (
    <div className={"resources-container tests"}>
      <ServicesDividerRoot className={"ServicesDivider tests"}>
        <GridDividerTestSvg style={{ marginLeft: "28px" }} />
        <ServicesLabel>All Resources</ServicesLabel>
      </ServicesDividerRoot>
      <OverviewGrid items={items} />
    </div>
  ) : null
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
  let testItems = allItems.filter((item) => item.isTest)

  return (
    <OverviewPaneRoot>
      <OverviewTabBar />
      <OverviewResourceBar view={props.view} />
      <ServicesContainer>
        {PinnedResources(pinnedItems)}
        {AllResources(allResources)}
        {TestResources(testItems)}
      </ServicesContainer>
    </OverviewPaneRoot>
  )
}
