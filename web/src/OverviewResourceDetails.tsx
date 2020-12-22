import React from "react"
import styled from "styled-components"
import OverviewLogPane from "./OverviewLogPane"

type OverviewResourceDetailsProps = {
  name: string
  view: Proto.webviewResource
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
      <ActionBar>
        <div>links</div>
        <div>copy pod</div>
      </ActionBar>
      <OverviewLogPane manifestName={name} />
    </OverviewResourceDetailsRoot>
  )
}
