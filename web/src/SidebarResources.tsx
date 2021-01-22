import React, { PureComponent } from "react"
import styled from "styled-components"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  SidebarItemAll,
  triggerUpdate,
} from "./SidebarItemView"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { useSidebarPin } from "./SidebarPin"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import { ResourceView } from "./types"

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

// note: this is a PureComponent but we're not currently getting much value out of its pureness
// https://app.clubhouse.io/windmill/story/9949/web-purecomponent-optimizations-seem-to-not-be-working
export class SidebarResources extends PureComponent<SidebarProps> {
  constructor(props: SidebarProps) {
    super(props)
    this.triggerSelected = this.triggerSelected.bind(this)
  }

  triggerSelected(action: string) {
    if (this.props.selected) {
      triggerUpdate(this.props.selected, action)
    }
  }

  render() {
    let pb = this.props.pathBuilder
    let totalAlerts = this.props.items
      .map((i) => i.buildAlertCount + i.runtimeAlertCount)
      .reduce((sum, current) => sum + current, 0)

    let listItems = this.props.items.map((item) => (
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
          items={this.props.items}
          pathBuilder={this.props.pathBuilder}
          onTrigger={this.triggerSelected}
          resourceView={this.props.resourceView}
        />
      </SidebarResourcesRoot>
    )
  }
}

export default SidebarResources
