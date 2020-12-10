import React from "react"
import styled from "styled-components"
import PathBuilder from "./PathBuilder"
import { Color } from "./style-helpers"

type OverviewResourceBarProps = {
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewResourceBarRoot = styled.div`
  display: flex;
  width: 100%;
  border-bottom: 1px dotted ${Color.grayLight};
  justify-content: center;
  align-items: center;
`

let ResourceSpecSummary = styled.div`
  display: flex;
  margin-left: 32px;
  flex-shrink: 1;
  width: 50%;
`

let ResourceBarEnd = styled.div`
  flex-shrink: 1;
  width: 50%;
`

let ResourceSpecItem = styled.div`
  padding-right: 16px;
`

let ResourceStatus = styled.div`
  display: flex;
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  border-radius: 4px;
  justify-content: center;
  align-items: center;
  flex-grow: 0;
  white-space: nowrap;
  margin: 8px;
  padding: 4px 16px;
`

export default function OverviewResourceBar(props: OverviewResourceBarProps) {
  return (
    <OverviewResourceBarRoot>
      <ResourceSpecSummary>
        <ResourceSpecItem>16 Resources</ResourceSpecItem>
        <ResourceSpecItem>3 Errors</ResourceSpecItem>
        <ResourceSpecItem>0 Warnings</ResourceSpecItem>
      </ResourceSpecSummary>
      <ResourceStatus>12/16 up</ResourceStatus>
      <ResourceBarEnd>&nbsp;</ResourceBarEnd>
    </OverviewResourceBarRoot>
  )
}
