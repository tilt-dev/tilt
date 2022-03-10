import React from "react"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
import HeaderBar from "./HeaderBar"
import OverviewTable from "./OverviewTable"
import { OverviewTableBulkActions } from "./OverviewTableBulkActions"
import { OverviewTableDisplayOptions } from "./OverviewTableDisplayOptions"
import { ResourceNameFilter } from "./ResourceNameFilter"
import StarredResourceBar, {
  starredResourcePropsFromView,
} from "./StarredResourceBar"
import { Color, SizeUnit, Width } from "./style-helpers"

type OverviewTablePaneProps = {
  view: Proto.webviewView
}

let OverviewTablePaneStyle = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.gray20};
`

const OverviewTableStickyNav = styled.div`
  background-color: ${Color.gray20};
  position: sticky;
  top: 0;
  z-index: 1000;
`

const OverviewTableMenu = styled.section`
  display: flex;
  flex-direction: row;
  align-items: center;
`

const OverviewTableResourceNameFilter = styled(ResourceNameFilter)`
  margin-left: ${SizeUnit(1 / 2)};
  margin-right: ${SizeUnit(1 / 2)};
  min-width: ${Width.sidebarDefault}px;
`

export default function OverviewTablePane(props: OverviewTablePaneProps) {
  return (
    <OverviewTablePaneStyle>
      <OverviewTableStickyNav>
        <HeaderBar view={props.view} currentPage={AnalyticsType.Grid} />
        <StarredResourceBar {...starredResourcePropsFromView(props.view, "")} />
        <OverviewTableMenu aria-label="Resource menu">
          <OverviewTableResourceNameFilter />
          <OverviewTableBulkActions uiButtons={props.view.uiButtons} />
          <OverviewTableDisplayOptions />
        </OverviewTableMenu>
      </OverviewTableStickyNav>
      <OverviewTable view={props.view} />
    </OverviewTablePaneStyle>
  )
}
