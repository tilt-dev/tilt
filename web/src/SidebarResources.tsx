import React, { Dispatch, PropsWithChildren, SetStateAction } from "react"
import styled from "styled-components"
import Features, { FeaturesContext, Flag } from "./feature"
import { orderLabels } from "./labels"
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
  props: PropsWithChildren<{ name: string }>
): JSX.Element {
  return (
    <div>
      <SidebarListSectionName>{props.name}</SidebarListSectionName>
      <SidebarListSectionItems>{props.children}</SidebarListSectionItems>
    </div>
  )
}

function SidebarItemsView(props: SidebarProps) {
  return (
    <>
      {props.items.map((item) => (
        <SidebarItemView
          key={"sidebarItem-" + item.name}
          item={item}
          selected={props.selected === item.name}
          pathBuilder={props.pathBuilder}
          resourceView={props.resourceView}
        />
      ))}
    </>
  )
}

function SidebarLabelListSection(props: { label: string } & SidebarProps) {
  if (props.items.length === 0) {
    return null
  }

  return (
    <>
      <SidebarListSectionName>{props.label}</SidebarListSectionName>
      <SidebarListSectionItems>
        <SidebarItemsView {...props} />
      </SidebarListSectionItems>
    </>
  )
}

function SidebarGroupedByLabels(props: SidebarProps) {
  const labelsToResources: { [key: string]: SidebarItem[] } = {}
  const unlabeled: SidebarItem[] = []

  props.items.forEach((item) => {
    if (item.labels.length) {
      item.labels.forEach((label) => {
        if (!labelsToResources.hasOwnProperty(label)) {
          labelsToResources[label] = []
        }

        labelsToResources[label].push(item)
      })
    } else {
      unlabeled.push(item)
    }
  })

  const labels = orderLabels(Object.keys(labelsToResources))

  return (
    <>
      {labels.map((label) => (
        <SidebarLabelListSection
          {...props}
          key={`sidebarItem-${label}`}
          label={label}
          items={labelsToResources[label]}
        />
      ))}
      <SidebarLabelListSection {...props} label="unlabeled" items={unlabeled} />
    </>
  )
}

type UIResource = Proto.v1alpha1UIResource
type Build = Proto.v1alpha1UIBuildTerminated

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

function resourcesHaveLabels(features: Features, items: SidebarItem[]) {
  if (!features.isEnabled(Flag.Labels)) {
    return false
  }

  return items.some((item) => item.labels.length > 0)
}

export class SidebarResources extends React.Component<SidebarProps> {
  constructor(props: SidebarProps) {
    super(props)
    this.triggerSelected = this.triggerSelected.bind(this)
  }

  static contextType = FeaturesContext

  triggerSelected() {
    if (this.props.selected) {
      triggerUpdate(this.props.selected)
    }
  }

  renderWithOptions(
    options: SidebarOptions,
    setOptions: Dispatch<SetStateAction<SidebarOptions>>
  ) {
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

    // only say no matches if there were actually items that got filtered out
    // otherwise, there might just be 0 resources because there are 0 resources
    // (though technically there's probably always at least a Tiltfile resource)
    const noResourcesMatchFilter =
      filteredItems.length === 0 && this.props.items.length !== 0
    const listItems = noResourcesMatchFilter ? (
      <NoMatchesFound key={"no-matches"}>No matching resources</NoMatchesFound>
    ) : (
      <SidebarItemsView {...this.props} items={filteredItems} />
    )

    let isOverviewClass =
      this.props.resourceView === ResourceView.OverviewDetail
        ? "isOverview"
        : ""

    // TODO: The above filtering and sorting logic will get refactored during more resource
    // grouping work. It won't be necessary to map and sort here, but within each group
    const labelsEnabled = resourcesHaveLabels(this.context, this.props.items)

    return (
      <SidebarResourcesRoot className={`Sidebar-resources ${isOverviewClass}`}>
        <SidebarList>
          <OverviewSidebarOptions
            showFilters={showFilters}
            options={options}
            setOptions={setOptions}
          />
          {labelsEnabled ? (
            <SidebarGroupedByLabels {...this.props} items={filteredItems} />
          ) : (
            <SidebarListSection name="resources">
              {listItems}
            </SidebarListSection>
          )}
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
