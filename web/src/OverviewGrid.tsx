import React from "react"
import styled from "styled-components"
import OverviewItemView, { OverviewItem } from "./OverviewItemView"
import PathBuilder from "./PathBuilder"

type OverviewGridProps = {
  view: Proto.webviewView
  pathBuilder: PathBuilder
}

let OverviewGridRoot = styled.ul`
  display: flex;
  flex-wrap: wrap;
  width: 100%;
  flex-grow: 1;
  align-content: flex-start;
`

export default function OverviewGrid(props: OverviewGridProps) {
  let resources = props.view.resources ?? []
  let itemViews = resources.map((res) => {
    let item = new OverviewItem(res)
    return (
      <OverviewItemView
        key={item.name}
        item={item}
        pathBuilder={props.pathBuilder}
      />
    )
  })
  return <OverviewGridRoot>{itemViews}</OverviewGridRoot>
}
