import React from "react"
import HeaderBar from "./HeaderBar"
import { OverviewPaneRoot } from "./OverviewPane"
import OverviewTable from "./OverviewTable"
import StarredResourceBar, {
  starredResourcePropsFromView,
} from "./StarredResourceBar"

export default function OverviewTablePane(props: { view: Proto.webviewView }) {
  return (
    <OverviewPaneRoot>
      <HeaderBar view={props.view} />
      <StarredResourceBar {...starredResourcePropsFromView(props.view, "")} />
      <OverviewTable view={props.view} />
    </OverviewPaneRoot>
  )
}
