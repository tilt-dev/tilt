import React from "react"
import styled from "styled-components"
import PathBuilder from "./PathBuilder"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"

type OverviewGridProps = {
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewGridRoot = styled.div`
  display: flex;
  flex-wrap: wrap;
  width: 100%;
  flex-grow: 1;
  align-content: flex-start;
`

let Card = styled.div`
  color: ${Color.white};
  background-color: ${Color.gray};
  display: flex;
  flex-direction: column;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid ${Color.gray};
  font-size: ${FontSize.small};
  font-family: ${Font.monospace};
  width: 330px;
  margin: ${SizeUnit(1)} ${SizeUnit(0.75)};
`

export default function OverviewGrid(props: OverviewGridProps) {
  return (
    <OverviewGridRoot>
      <Card>Resource 1</Card>
      <Card>Resource 2</Card>
    </OverviewGridRoot>
  )
}
