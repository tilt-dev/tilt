import React from "react"
import styled from "styled-components"
import OverviewTabBar from "./OverviewTabBar"
import PathBuilder from "./PathBuilder"

type OverviewResourceDetailsProps = {
  view: Proto.webviewResource
  pathBuilder: PathBuilder
}

let OverviewResourceDetailsRoot = styled.div`
  display: flex;
  flex-grow: 1;
  flex-direction: column;
`

let ActionBar = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
`

// TODO(nick): it's not clear to me how much of the current LogPane
// we can re-use for this view. It might be easiest to fork it.
let LogPane = styled.div`
  overflow: auto;
  flex-grow: 1;
`

export default function OverviewResourceDetails(
  props: OverviewResourceDetailsProps
) {
  return (
    <OverviewResourceDetailsRoot>
      <OverviewTabBar pathBuilder={props.pathBuilder} tabsOnly={true} />
      <ActionBar>
        <div>links</div>
        <div>copy pod</div>
      </ActionBar>
      <LogPane>
        <div>log line 1</div>
        <div>log line 2</div>
        <div>log line 3</div>
        <div>log line 4</div>
        <div>log line 5</div>
        <div>log line 6</div>
      </LogPane>
    </OverviewResourceDetailsRoot>
  )
}
