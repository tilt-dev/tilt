import React from "react"
import styled from "styled-components"
import HeaderBar from "./HeaderBar"
import OverviewTable from "./OverviewTable"
import StarredResourceBar, {
  starredResourcePropsFromView,
} from "./StarredResourceBar"
import { Color } from "./style-helpers"

type OverviewTablePaneProps = {
  view: Proto.webviewView
}

let OverviewTablePaneStyle = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
`

export default function OverviewTablePane(props: OverviewTablePaneProps) {
  return (
    <OverviewTablePaneStyle>
      <HeaderBar view={props.view} />
      <StarredResourceBar {...starredResourcePropsFromView(props.view, "")} />
      <OverviewTable view={props.view} />
    </OverviewTablePaneStyle>
  )
}
