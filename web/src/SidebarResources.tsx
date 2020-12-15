import React, { PureComponent, useContext } from "react"
import styled from "styled-components"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  SidebarItemAll,
  triggerUpdate,
} from "./SidebarItemView"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { sidebarPinContext, SidebarPinContextProvider } from "./SidebarPin"
import { Color, FontSize, SizeUnit, Width } from "./style-helpers"
import { ResourceView } from "./types"

let SidebarResourcesRoot = styled.nav`
  flex: 1 0 auto;
  margin-left: ${SizeUnit(0.2)};
  margin-right: ${SizeUnit(0.2)};
`
let SidebarList = styled.div``

let SidebarListSectionName = styled.div`
  width: ${Width.sidebar - Width.sidebarTriggerButton - 1}px;
  margin-left: ${Width.sidebarPinButton}px;
  text-transform: uppercase;
  color: ${Color.grayLight};
  font-size: ${FontSize.small};
`
const SidebarListSectionItems = styled.ul`
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
  initialPinnedItemsForTesting?: string[]
}

function SidebarResources(props: SidebarProps) {
  return (
    <SidebarPinContextProvider
      initialValueForTesting={props.initialPinnedItemsForTesting}
    >
      <PureSidebarResources {...props} />
    </SidebarPinContextProvider>
  )
}

function PinnedItems(props: SidebarProps) {
  let ctx = useContext(sidebarPinContext)
  let pinnedItems = ctx.pinnedResources?.flatMap((r) =>
    props.items
      .filter((i) => i.name === r)
      .map((i) =>
        SidebarItemView({
          item: i,
          selected: props.selected === i.name,
          renderPin: false,
          pathBuilder: props.pathBuilder,
          resourceView: props.resourceView,
        })
      )
  )

  if (!pinnedItems?.length) {
    return null
  }

  return <SidebarListSection name="favorites">{pinnedItems}</SidebarListSection>
}

// note: this is a PureComponent but we're not currently getting much value out of its pureness
// https://app.clubhouse.io/windmill/story/9949/web-purecomponent-optimizations-seem-to-not-be-working
class PureSidebarResources extends PureComponent<SidebarProps> {
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

    let allLink =
      this.props.resourceView === ResourceView.Alerts
        ? pb.path("/alerts")
        : pb.path("/")

    let totalAlerts = this.props.items
      .map((i) => i.buildAlertCount + i.runtimeAlertCount)
      .reduce((sum, current) => sum + current, 0)

    let listItems = this.props.items.map((item) =>
      SidebarItemView({
        item: item,
        selected: this.props.selected === item.name,
        renderPin: true,
        pathBuilder: this.props.pathBuilder,
        resourceView: this.props.resourceView,
      })
    )

    let nothingSelected = !this.props.selected

    return (
      <SidebarResourcesRoot className="Sidebar-resources">
        <SidebarList>
          <SidebarListSection name="">
            <SidebarItemAll
              nothingSelected={nothingSelected}
              allLink={allLink}
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
        />
      </SidebarResourcesRoot>
    )
  }
}

export default SidebarResources
