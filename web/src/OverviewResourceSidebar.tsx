import React from "react"
import styled from "styled-components"
import OverviewTabBar from "./OverviewTabBar"
import PathBuilder from "./PathBuilder"

type OverviewResourceSidebarProps = {
  name: string
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewResourceSidebarRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 400px;
`

export default function OverviewResourceSidebar(
  props: OverviewResourceSidebarProps
) {
  return (
    <OverviewResourceSidebarRoot>
      <OverviewTabBar {...props} logoOnly={true} />
      <div>16 Resources</div>
      <div>3 errors | 0 warnings</div>
      <div>All</div>
      <div>recservice</div>
      <div>paymentservice</div>
    </OverviewResourceSidebarRoot>
  )
}
