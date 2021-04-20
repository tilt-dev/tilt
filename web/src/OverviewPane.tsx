import React, { useEffect } from "react"
import styled from "styled-components"
import { ReactComponent as GridDividerAllSvg } from "./assets/svg/grid-divider-all.svg"
import { ReactComponent as GridDividerTestSvg } from "./assets/svg/grid-divider-test.svg"
import HeaderBar from "./HeaderBar"
import OverviewGrid from "./OverviewGrid"
import { OverviewItem } from "./OverviewItemView"
import StarredResourceBar, {
  starredResourcePropsFromView,
} from "./StarredResourceBar"
import { useStarredResources } from "./StarredResourcesContext"
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

type ResourceProps = {
  items: OverviewItem[]
}

export function AllResources(props: ResourceProps) {
  return props.items?.length ? (
    <React.Fragment>
      <ServicesDividerRoot>
        <GridDividerAllSvg style={{ marginLeft: "28px" }} />
        <ServicesLabel>All Resources</ServicesLabel>
      </ServicesDividerRoot>
      <OverviewGrid items={props.items} />
    </React.Fragment>
  ) : null
}

export function TestResources(props: ResourceProps) {
  return props.items?.length ? (
    <React.Fragment>
      <ServicesDividerRoot>
        <GridDividerTestSvg style={{ marginLeft: "28px" }} />
        <ServicesLabel>Tests</ServicesLabel>
      </ServicesDividerRoot>
      <OverviewGrid items={props.items} />
    </React.Fragment>
  ) : null
}

export default function OverviewPane(props: OverviewPaneProps) {
  let starContext = useStarredResources()
  let resources = props.view.resources || []
  let allItems = resources.map((res) => new OverviewItem(res))
  let allResources = allItems.filter(
    (item) =>
      // NOTE(maia): this is gross naming, but until we have better nouns:
      //  "all resources" = everything but tests
      // (This is bad because in the backend, tests are also "resources", but  ¯\_(ツ)_/¯ )
      !item.isTest
  )
  let starredItems = allItems.filter((item) =>
    starContext.starredResources.includes(item.name)
  )
  let testItems = allItems.filter((item) => item.isTest)

  // Hide the HTML element scrollbars, since this pane does all scrolling internally.
  // TODO(nick): Remove this when the old UI is deleted.
  useEffect(() => {
    document.documentElement.style.overflow = "hidden"
  })

  return (
    <OverviewPaneRoot>
      <HeaderBar view={props.view} />
      <StarredResourceBar {...starredResourcePropsFromView(props.view, "")} />
      <ServicesContainer>
        <AllResources items={allResources} />
        <TestResources items={testItems} />
      </ServicesContainer>
    </OverviewPaneRoot>
  )
}
