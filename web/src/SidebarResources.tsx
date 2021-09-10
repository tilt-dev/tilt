import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import React, {
  ChangeEvent,
  Dispatch,
  PropsWithChildren,
  SetStateAction,
} from "react"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
import { FeaturesContext, Flag } from "./feature"
import { GlobalOptions } from "./GlobalOptionsContext"
import {
  GroupByLabelView,
  orderLabels,
  TILTFILE_LABEL,
  UNLABELED_LABEL,
} from "./labels"
import { PersistentStateProvider } from "./LocalStorage"
import { OverviewSidebarOptions } from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import {
  AccordionDetailsStyleResetMixin,
  AccordionStyleResetMixin,
  AccordionSummaryStyleResetMixin,
  ResourceGroupsInfoTip,
  ResourceGroupSummaryIcon,
  ResourceGroupSummaryMixin,
} from "./ResourceGroups"
import { useResourceGroups } from "./ResourceGroupsContext"
import { matchesResourceName } from "./ResourceNameFilter"
import { SidebarGroupStatusSummary } from "./ResourceStatusSummary"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  SidebarItemRoot,
  triggerUpdate,
} from "./SidebarItemView"
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

let SidebarList = styled.div`
  margin-bottom: ${SizeUnit(1.75)};
`

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

const SidebarLabelSection = styled(Accordion)`
  ${AccordionStyleResetMixin}

  /* Set specific margins for sidebar */
  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded {
    margin: ${SizeUnit(1 / 3)} ${SizeUnit(1 / 2)};
  }
`

const SidebarGroupSummary = styled(AccordionSummary)`
  ${AccordionSummaryStyleResetMixin}
  ${ResourceGroupSummaryMixin}

  /* Set specific background and borders for sidebar */
  .MuiAccordionSummary-content {
    background-color: ${Color.grayLighter};
    border: 1px solid ${Color.grayLight};
    border-radius: ${SizeUnit(1 / 8)};
    font-size: ${FontSize.small};
  }
`

const SidebarGroupName = styled.span`
  margin-right: auto;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
`

const SidebarGroupDetails = styled(AccordionDetails)`
  ${AccordionDetailsStyleResetMixin}

  &.MuiAccordionDetails-root {
    ${SidebarItemRoot} {
      margin-right: unset;
    }
  }
`

const GROUP_INFO_TOOLTIP_ID = "sidebar-groups-info"

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

function SidebarItemsView(props: SidebarProps & { groupView?: boolean }) {
  return (
    <>
      {props.items.map((item) => (
        <SidebarItemView
          key={"sidebarItem-" + item.name}
          groupView={props.groupView}
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

  const formattedLabel =
    props.label === UNLABELED_LABEL ? <em>{props.label}</em> : props.label
  const labelNameId = `sidebarItem-${props.label}`

  const { getGroup, toggleGroupExpanded } = useResourceGroups()
  const { expanded } = getGroup(props.label)
  const handleChange = (_e: ChangeEvent<{}>) =>
    toggleGroupExpanded(props.label, AnalyticsType.Detail)

  // TODO (lizz): Improve the accessibility interface for accordion feature by adding focus styles
  // according to https://www.w3.org/TR/wai-aria-practices-1.1/examples/accordion/accordion.html
  return (
    <SidebarLabelSection expanded={expanded} onChange={handleChange}>
      <SidebarGroupSummary id={labelNameId}>
        <ResourceGroupSummaryIcon role="presentation" />
        <SidebarGroupName>{formattedLabel}</SidebarGroupName>
        <SidebarGroupStatusSummary
          aria-label={`Status summary for ${props.label} group`}
          resources={props.items}
        />
      </SidebarGroupSummary>
      <SidebarGroupDetails aria-labelledby={labelNameId}>
        <SidebarListSectionItems>
          <SidebarItemsView {...props} />
        </SidebarListSectionItems>
      </SidebarGroupDetails>
    </SidebarLabelSection>
  )
}

function resourcesLabelView(
  items: SidebarItem[]
): GroupByLabelView<SidebarItem> {
  const labelsToResources: { [key: string]: SidebarItem[] } = {}
  const unlabeled: SidebarItem[] = []
  const tiltfile: SidebarItem[] = []

  items.forEach((item) => {
    if (item.labels.length) {
      item.labels.forEach((label) => {
        if (!labelsToResources.hasOwnProperty(label)) {
          labelsToResources[label] = []
        }

        labelsToResources[label].push(item)
      })
    } else if (item.isTiltfile) {
      tiltfile.push(item)
    } else {
      unlabeled.push(item)
    }
  })

  // Labels are always displayed in sorted order
  const labels = orderLabels(Object.keys(labelsToResources))

  return { labels, labelsToResources, tiltfile, unlabeled }
}

function SidebarGroupedByLabels(props: SidebarProps) {
  const { labels, labelsToResources, tiltfile, unlabeled } = resourcesLabelView(
    props.items
  )

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
      <SidebarLabelListSection
        {...props}
        label={UNLABELED_LABEL}
        items={unlabeled}
      />
      <SidebarListSection name={TILTFILE_LABEL}>
        <SidebarItemsView {...props} items={tiltfile} groupView={true} />
      </SidebarListSection>
    </>
  )
}

type SidebarProps = {
  items: SidebarItem[]
  selected: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
  globalOptions: GlobalOptions
}

export const defaultOptions: SidebarOptions = {
  alertsOnTop: false,
}

// Note: non-nullable fields added to SidebarOptions after its initial release
// need to have default values filled in here
function MaybeUpgradeSavedSidebarOptions(savedOptions: SidebarOptions) {
  // Since `resourceNameFilter` has moved out of SidebarOptions and into
  // GlobalOptions, do not include it in the saved state
  if (savedOptions.hasOwnProperty("resourceNameFilter")) {
    const updatedOptions = { ...defaultOptions }
    Object.keys(savedOptions).forEach((option) => {
      if (option !== "resourceNameFilter") {
        updatedOptions[option as keyof SidebarOptions] =
          savedOptions[option as keyof SidebarOptions]
      }
    })

    return updatedOptions
  }

  return savedOptions
}

function hasAlerts(item: SidebarItem): boolean {
  return item.buildAlertCount > 0 || item.runtimeAlertCount > 0
}

function sortByHasAlerts(itemA: SidebarItem, itemB: SidebarItem): number {
  return Number(hasAlerts(itemB)) - Number(hasAlerts(itemA))
}

function applyOptionsToItems(
  items: SidebarItem[],
  options: SidebarOptions & GlobalOptions
): SidebarItem[] {
  let itemsToDisplay: SidebarItem[] = [...items]
  if (options.resourceNameFilter) {
    itemsToDisplay = itemsToDisplay.filter((item) =>
      matchesResourceName(item.name, options.resourceNameFilter)
    )
  }

  if (options.alertsOnTop) {
    itemsToDisplay.sort(sortByHasAlerts)
  }

  return itemsToDisplay
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
    sidebarOptions: SidebarOptions,
    setSidebarOptions: Dispatch<SetStateAction<SidebarOptions>>
  ) {
    const options = { ...sidebarOptions, ...this.props.globalOptions }
    const filteredItems = applyOptionsToItems(this.props.items, options)

    // only say no matches if there were actually items that got filtered out
    // otherwise, there might just be 0 resources because there are 0 resources
    // (though technically there's probably always at least a Tiltfile resource)
    const resourceFilterApplied =
      this.props.globalOptions.resourceNameFilter.length > 0
    const noResourcesMatchFilter =
      resourceFilterApplied && filteredItems.length === 0
    const listItems = noResourcesMatchFilter ? (
      <NoMatchesFound key={"no-matches"}>No matching resources</NoMatchesFound>
    ) : (
      <SidebarItemsView {...this.props} items={filteredItems} />
    )
    const sidebarName = resourceFilterApplied
      ? `${filteredItems.length} result${filteredItems.length === 1 ? "" : "s"}`
      : "resources"

    let isOverviewClass =
      this.props.resourceView === ResourceView.OverviewDetail
        ? "isOverview"
        : ""

    const labelsEnabled: boolean = this.context.isEnabled(Flag.Labels)
    const resourcesHaveLabels = this.props.items.some(
      (item) => item.labels.length > 0
    )

    // The label group tip is only displayed if labels are enabled but not used
    const displayLabelGroupsTip = labelsEnabled && !resourcesHaveLabels
    // The label group view does not display if a resource name filter is applied
    const displayLabelGroups =
      !resourceFilterApplied && labelsEnabled && resourcesHaveLabels

    return (
      <SidebarResourcesRoot className={`Sidebar-resources ${isOverviewClass}`}>
        {displayLabelGroupsTip && (
          <ResourceGroupsInfoTip idForIcon={GROUP_INFO_TOOLTIP_ID} />
        )}
        <SidebarList
          aria-describedby={
            displayLabelGroupsTip ? GROUP_INFO_TOOLTIP_ID : undefined
          }
        >
          <OverviewSidebarOptions
            options={sidebarOptions}
            setOptions={setSidebarOptions}
          />
          {displayLabelGroups ? (
            <SidebarGroupedByLabels {...this.props} items={filteredItems} />
          ) : (
            <SidebarListSection name={sidebarName}>
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
