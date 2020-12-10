import React from "react"
import styled from "styled-components"
import PathBuilder from "./PathBuilder"
import { Color } from "./style-helpers"

type OverviewTabBarProps = {
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewTabBarRoot = styled.div`
  display: flex;
  width: 100%;
  height: 68px;
  background-color: ${Color.gray};
  border-bottom: 1px solid ${Color.grayLight};
`

let Tab = styled.div`
  border: 1px solid ${Color.grayLight};
  border-radius: 4px 4px 0px 0px;
  margin: 12px;
  flex-grow: 0;
  padding: 8px;
`

export default function OverviewTabBar(props: OverviewTabBarProps) {
  return (
    <OverviewTabBarRoot>
      <Tab>Logo</Tab>
      <Tab>Tab 1</Tab>
      <Tab>Tab 2</Tab>
    </OverviewTabBarRoot>
  )
}
