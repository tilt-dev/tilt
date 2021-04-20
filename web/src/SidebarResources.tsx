import React, { Dispatch, SetStateAction } from "react"
import styled from "styled-components"
import { PersistentStateProvider } from "./LocalStorage"
import { OverviewSidebarOptions } from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView, { triggerUpdate } from "./SidebarItemView"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
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

const NoMatchesFound = styled.li`
  margin-left: ${SizeUnit(0.5)};
  color: ${Color.grayLightest};
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

export const defaultOptions: SidebarOptions = {
  testsHidden: false,
  testsOnly: false,
  alertsOnTop: false,
  resourceNameFilter: "",
}

function MaybeUpgradeSavedSidebarOptions(o: SidebarOptions) {
  // non-nullable fields added to SidebarOptions after its initial release need to have default values
  // filled in here
  return { ...o, resourceNameFilter: o.resourceNameFilter ?? "" }
}

function hasAlerts(item: SidebarItem): boolean {
  return item.buildAlertCount > 0 || item.runtimeAlertCount > 0
}

function sortByHasAlerts(itemA: SidebarItem, itemB: SidebarItem): number {
  return Number(hasAlerts(itemB)) - Number(hasAlerts(itemA))
}

function matchesResourceName(item: SidebarItem, filter: string): boolean {
  filter = filter.trim()
  // this is functionally redundant but probably an important enough case to make its own thing
  if (filter === "") {
    return true
  }
  // a resource matches the query if the resource name contains all tokens in the query
  return filter
    .split(" ")
    .every((token) => item.name.toLowerCase().includes(token.toLowerCase()))
}

export class SidebarResources extends React.Component<SidebarProps> {
  constructor(props: SidebarProps) {
    super(props)
    this.triggerSelected = this.triggerSelected.bind(this)
  }

  triggerSelected() {
    if (this.props.selected) {
      triggerUpdate(this.props.selected)
    }
  }

  renderWithOptions(
    options: SidebarOptions,
    setOptions: Dispatch<SetStateAction<SidebarOptions>>
  ) {
    let pb = this.props.pathBuilder
    let totalAlerts = this.props.items // Open Q: do we include alert totals for hidden elems?
      .map((i) => i.buildAlertCount + i.runtimeAlertCount)
      .reduce((sum, current) => sum + current, 0)

    // generally, only show filters if there are tests (otherwise the filters are just noise)
    //   however, also show filters if the filter options are non-default
    //   (e.g., in case there were previously tests and the user deselected resources)
    const showFilters =
      this.props.items.some((item) => item.isTest) ||
      options.testsHidden !== defaultOptions.testsHidden ||
      options.testsOnly !== defaultOptions.testsOnly

    let filteredItems = [...this.props.items]
    if (options.testsHidden) {
      filteredItems = this.props.items.filter((item) => !item.isTest)
    } else if (options.testsOnly) {
      filteredItems = this.props.items.filter((item) => item.isTest)
    }

    filteredItems = filteredItems.filter((item) =>
      matchesResourceName(item, options.resourceNameFilter)
    )

    if (options.alertsOnTop) {
      filteredItems.sort(sortByHasAlerts)
    }

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

    // only say no matches if there were actually items that got filtered out
    // otherwise, there might just be 0 resources because there are 0 resources
    // (though technically there's probably always at least a Tiltfile resource)
    if (listItems.length === 0 && this.props.items.length !== 0) {
      listItems = [
        <NoMatchesFound key={"no-matches"}>
          No matching resources
        </NoMatchesFound>,
      ]
    }

    return (
      <SidebarResourcesRoot className={`Sidebar-resources ${isOverviewClass}`}>
        <SidebarList>
          <OverviewSidebarOptions
            showFilters={showFilters}
            options={options}
            setOptions={setOptions}
          />
          <SidebarListSection name="resources">{listItems}</SidebarListSection>
        </SidebarList>
        <SidebarKeyboardShortcuts
          selected={this.props.selected}
          items={filteredItems}
          onTrigger={this.triggerSelected}
          resourceView={this.props.resourceView}
        />
      </SidebarResourcesRoot>
    )
  }

  render() {
    return (
      <PersistentStateProvider
        defaultValue={defaultOptions}
        name={"sidebar_options"}
        maybeUpgradeSavedState={MaybeUpgradeSavedSidebarOptions}
      >
        {(value: SidebarOptions, set) => this.renderWithOptions(value, set)}
      </PersistentStateProvider>
    )
  }
}

export default SidebarResources
