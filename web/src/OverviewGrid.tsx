import React from "react"
import styled from "styled-components"
import OverviewItemView, { OverviewItem } from "./OverviewItemView"
import { SizeUnit } from "./style-helpers"

type OverviewGridProps = {
  view: Proto.webviewView
}

let OverviewGridRoot = styled.ul`
  display: flex;
  flex-wrap: wrap;
  width: 100%;
  flex-grow: 1;
  flex-shrink: 1;
  align-content: flex-start;
  overflow: auto;
  padding-left: ${SizeUnit(0.25)};
  position: relative;
  box-sizing: border-box;
`

export default function OverviewGrid(props: OverviewGridProps) {
  let resources = props.view.resources ?? []
  let itemViews = resources.map((res) => {
    let item = new OverviewItem(res)
    return <OverviewItemView key={item.name} item={item} />
  })
  return <OverviewGridRoot>{itemViews}</OverviewGridRoot>
}
