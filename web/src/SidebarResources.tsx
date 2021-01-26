import React from "react"
import styled from "styled-components"
import { SidebarOptionsSetter } from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  SidebarItemAll,
  triggerUpdate,
} from "./SidebarItemView"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { useSidebarPin } from "./SidebarPin"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import { ResourceView, SidebarOptions } from "./types"

let SidebarResourcesRoot = styled.nav`
  flex: 1 0 auto;

  &.isOverview {
    overflow: auto;
    flex-shrink: 1;
  }
`

let SidebarList = styled.div``

let SidebarListSectionName = styled.div`
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(0.5)};
  text-transform: uppercase;
  color: ${Color.grayLight};
  font-size: ${FontSize.small};
`
const SidebarListSectionItems = styled.ul`
  margin-top: ${SizeUnit(0.25)};
  list-style: none;
`

export function SidebarListSection(
  props: React.PropsWithChildren<{ name: string }>
): JSX.Element {
  return (
    <div>
      <SidebarListSectionName>{props.name}</SidebarListSectionName>
      <SidebarListSectionItems>{props.children}</SidebarListSectionItems>
    </div>
  )
}

type Resource = Proto.webviewResource
type Build = Proto.webviewBuildRecord

type SidebarProps = {
  items: SidebarItem[]
  selected: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
}

type SidebarState = SidebarOptions

let defaultFilters: SidebarOptions = {
  showResources: true,
  showTests: true,
}

function PinnedItems(props: SidebarProps) {
  let ctx = useSidebarPin()
  let pinnedItems = ctx.pinnedResources?.flatMap((r) =>
    props.items
      .filter((i) => i.name === r)
      .map((i) => (
        <SidebarItemView
          key={"sidebarItemPinned-" + i.name}
          item={i}
          selected={props.selected === i.name}
          pathBuilder={props.pathBuilder}
          resourceView={props.resourceView}
        />
      ))
  )

  if (!pinnedItems?.length) {
    return null
  }

  return <SidebarListSection name="Pinned">{pinnedItems}</SidebarListSection>
}

export class SidebarResources extends React.Component<
  SidebarProps,
  SidebarState
> {
  constructor(props: SidebarProps) {
    super(props)
    this.triggerSelected = this.triggerSelected.bind(this)
    this.toggleShowResources = this.toggleShowResources.bind(this)
    this.toggleShowTests = this.toggleShowTests.bind(this)
    this.state = defaultFilters
  }

  triggerSelected(action: string) {
    if (this.props.selected) {
      triggerUpdate(this.props.selected, action)
    }
  }

  toggleShowResources() {
    this.setState((prevState: SidebarOptions) => {
      return {
        showResources: !prevState.showResources,
      }
    })
  }

  toggleShowTests() {
    this.setState((prevState: SidebarOptions) => {
      return {
        showTests: !prevState.showTests,
      }
    })
  }

  render() {
    let pb = this.props.pathBuilder
    let totalAlerts = this.props.items // Open Q: do we include alert totals for hidden elems?
      .map((i) => i.buildAlertCount + i.runtimeAlertCount)
      .reduce((sum, current) => sum + current, 0)

    // TODO: what do we do when we filter out the selected item? Pinned item(s)?
    //       and what effect does this have on keyboard shortcuts? :(
    let filteredItems = this.props.items.filter(
      (item) =>
        (!item.isTest && this.state.showResources) ||
        (item.isTest && this.state.showTests) ||
        item.isTiltfile
    )

    let listItems = filteredItems.map((item) => (
      <SidebarItemView
        key={"sidebarItem-" + item.name}
        item={item}
        selected={this.props.selected == item.name}
        pathBuilder={this.props.pathBuilder}
        resourceView={this.props.resourceView}
      />
    ))

    let nothingSelected = !this.props.selected
    let isOverviewClass =
      this.props.resourceView === ResourceView.OverviewDetail
        ? "isOverview"
        : ""

    return (
      <SidebarResourcesRoot className={`Sidebar-resources ${isOverviewClass}`}>
        <SidebarList>
          <SidebarOptionsSetter
            curState={this.state}
            toggleShowResources={this.toggleShowResources}
            toggleShowTests={this.toggleShowTests}
          />
          <SidebarListSection name="">
            <SidebarItemAll
              nothingSelected={nothingSelected}
              totalAlerts={totalAlerts}
            />
          </SidebarListSection>
          <PinnedItems {...this.props} />
          <SidebarListSection name="resources">{listItems}</SidebarListSection>
        </SidebarList>
        <SidebarKeyboardShortcuts
          selected={this.props.selected}
          items={filteredItems}
          pathBuilder={this.props.pathBuilder}
          onTrigger={this.triggerSelected}
          resourceView={this.props.resourceView}
        />
      </SidebarResourcesRoot>
    )
  }
}

export default SidebarResources
