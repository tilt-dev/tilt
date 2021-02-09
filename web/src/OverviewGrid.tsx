import React from "react"
import styled from "styled-components"
import OverviewItemView, { OverviewItem } from "./OverviewItemView"
import { Color, SizeUnit } from "./style-helpers"

type OverviewGridProps = {
  items: OverviewItem[]
}

let OverviewGridRoot = styled.ul`
  display: flex;
  flex-wrap: wrap;
  width: 100%;
  align-content: flex-start;
  overflow: auto;
  padding-left: ${SizeUnit(0.25)};
  position: relative;
  box-sizing: border-box;
  border-bottom: 1px dotted ${Color.grayLight};
  &:last-child {
    border-bottom-width: 0;
  }
`

export default function OverviewGrid(props: OverviewGridProps) {
  let itemViews = props.items.map((item) => {
    return <OverviewItemView key={item.name} item={item} />
  })
  return <OverviewGridRoot>{itemViews}</OverviewGridRoot>
}
